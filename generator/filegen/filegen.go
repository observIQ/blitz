package filegen

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/generator/ctime"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

// packageSourcePrefix is the explicit prefix that forces a source to be
// resolved against the data library (ignoring disk paths). Bare names
// (no slash, no prefix) also try the data library, but fall back to disk
// on miss; this prefix skips the fallback.
const packageSourcePrefix = "package:"

// sourceMode tracks which backend a generator instance resolved its
// source to. Set once in Start; workers read accordingly.
type sourceMode int

const (
	sourceModeDisk sourceMode = iota
	sourceModeLibrary
)

const componentName = "file"

// Cache provides thread-safe access to file line caches with optional TTL
type Cache struct {
	lruCache *expirable.LRU[string, []string]
	enabled  bool
}

// NewCache creates a new Cache with the given size limit and TTL
// ttl of 0 means entries never expire
func NewCache(enabled bool, ttl time.Duration, maxSize int) (*Cache, error) {
	if !enabled {
		return &Cache{enabled: false}, nil
	}

	// For 0 TTL (never expire), use a very large duration
	// expirable.LRU requires a TTL, so we use a large value for "never expire"
	effectiveTTL := ttl
	if ttl == 0 {
		effectiveTTL = 24 * 365 * time.Hour // ~1 year (effectively never)
	}

	lruCache := expirable.NewLRU[string, []string](maxSize, nil, effectiveTTL)

	return &Cache{
		lruCache: lruCache,
		enabled:  true,
	}, nil
}

// Get retrieves a cache entry if it exists and hasn't expired
func (c *Cache) Get(key string) ([]string, bool) {
	if !c.enabled {
		return nil, false
	}

	return c.lruCache.Get(key)
}

// Set stores a cache entry
func (c *Cache) Set(key string, lines []string) {
	if !c.enabled {
		return
	}

	c.lruCache.Add(key, lines)
}

// FileLogGenerator generates log data by reading from files
type FileLogGenerator struct {
	embed.ProducerMarker

	logger      *zap.Logger
	workers     int
	rate        time.Duration
	source      string // file path or directory path or glob pattern
	consumer    embed.LogConsumer
	dataLibrary fs.FS // optional; nil falls back to ./data_library on disk for "package:" / bare-name sources
	stopCh      chan struct{}
	tracker     *count.Tracker
	wg          sync.WaitGroup

	// resolved backend; set in Start
	mode sourceMode

	// File cache
	cache *Cache
}

// New creates a new File log generator. The consumer receives each
// generated record as a size-1 batch via ConsumeLogs.
//
// dataLibrary is optional. When non-nil, "package:" and bare-name
// sources are resolved against it (typical use: pass
// embeddedlibrary.FS() from github.com/observiq/blitz/generator/filegen/embeddedlibrary
// when running blitz embedded inside another binary). When nil, those
// sources resolve against the on-disk ./data_library/ directory, which
// is the standalone CLI behavior.
func New(logger *zap.Logger, workers int, rate time.Duration, source string, cacheEnabled bool, cacheTTL time.Duration, consumer embed.LogConsumer, dataLibrary fs.FS) (*FileLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	if rate <= 0 {
		return nil, fmt.Errorf("rate must be positive, got %v", rate)
	}

	if source == "" {
		return nil, fmt.Errorf("source cannot be empty")
	}

	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}

	cache, err := NewCache(cacheEnabled, cacheTTL, 1000) // 1000 is max size for LRU
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	return &FileLogGenerator{
		logger:      logger,
		workers:     workers,
		rate:        rate,
		source:      source,
		consumer:    consumer,
		dataLibrary: dataLibrary,
		stopCh:      make(chan struct{}),
		cache:       cache,
	}, nil
}

// Name returns the module identifier.
func (g *FileLogGenerator) Name() string { return componentName }

// Start starts the File log generator and launches workers that push
// records to the configured consumer.
func (g *FileLogGenerator) Start(_ context.Context) error {
	g.logger.Info("Starting File log generator",
		zap.String("source", g.source),
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate))

	// Record initial active workers count
	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)

	// Get list of files to read
	files, err := g.getFiles()
	if err != nil {
		return fmt.Errorf("get files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found to read")
	}

	g.logger.Info("Found files to read", zap.Int("count", len(files)))

	// Start workers
	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, files)
	}

	return nil
}

// Stop stops the File log generator
func (g *FileLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping File log generator")

	generator.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, componentName)

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("All workers stopped gracefully")
	case <-ctx.Done():
		g.logger.Warn("Context cancelled, some workers may not have stopped gracefully")
		return ctx.Err()
	}

	return nil
}

// getFiles returns a list of files to read and sets g.mode to indicate
// which backend (disk or data library) the files live in.
//
// Resolution order:
//  1. Source with "package:" prefix → library backend only (no disk fallback).
//  2. Source with a path separator or matching disk file/glob → disk backend.
//  3. Bare name (no separator, disk lookup fails) → library backend.
//
// When a bare name resolves to the library AND a same-named directory
// exists on disk relative to cwd, a warning is logged so users with
// unintentional collisions know to use the explicit "package:" prefix.
func (g *FileLogGenerator) getFiles() ([]string, error) {
	if strings.HasPrefix(g.source, packageSourcePrefix) {
		name := strings.TrimPrefix(g.source, packageSourcePrefix)
		files, err := g.libraryFiles(name)
		if err != nil {
			if g.libraryMissing() {
				return nil, g.errLibraryNotFound()
			}
			return nil, fmt.Errorf("package %q not found in the data library: %w", name, err)
		}
		g.mode = sourceModeLibrary
		return files, nil
	}

	files, diskErr := g.getFilesFromAutoDetect()
	if diskErr == nil {
		g.mode = sourceModeDisk
		return files, nil
	}

	// Bare name (no separator) — try the library as a fallback.
	if !strings.ContainsAny(g.source, "/\\") {
		libFiles, libErr := g.libraryFiles(g.source)
		if libErr == nil {
			g.warnIfCollision(g.source)
			g.mode = sourceModeLibrary
			return libFiles, nil
		}
		if g.libraryMissing() {
			return nil, g.errLibraryNotFound()
		}
		return nil, fmt.Errorf("source %q not found: not a file, directory, or glob on disk, and not a package in the data library", g.source)
	}

	return nil, diskErr
}

// libraryFS returns the data library backend: the on-disk library layered
// over the embedded one, disk winning per path (PIPE-1445). Either layer
// may be absent; with neither, it returns the nfpm path so a read surfaces
// a clear not-found error.
func (g *FileLogGenerator) libraryFS() fs.FS {
	disk, _ := diskLibrary()
	if fsys := overlayLibrary(disk, g.dataLibrary); fsys != nil {
		return fsys
	}
	return os.DirFS("/usr/share/blitz/data_library")
}

// libraryMissing reports whether no library is resolvable: no probe dir and
// an empty or absent embedded FS.
func (g *FileLogGenerator) libraryMissing() bool {
	entries, err := fs.ReadDir(g.libraryFS(), ".")
	return err != nil || len(entries) == 0
}

// errLibraryNotFound is the explicit "no library anywhere" error, naming
// where it looked and how to supply one.
func (g *FileLogGenerator) errLibraryNotFound() error {
	return fmt.Errorf("filegen data library not found: checked $BLITZ_DATA_LIBRARY_DIR, " +
		"./data_library, generator/filegen/embeddedlibrary/data_library, and " +
		"/usr/share/blitz/data_library, and no embedded library is compiled in; " +
		"install the blitz package, run from a repo checkout, or set " +
		"BLITZ_DATA_LIBRARY_DIR to a data_library directory")
}

// libraryFiles walks the named entry inside the library FS and returns
// the list of file paths (relative to the library root).
func (g *FileLogGenerator) libraryFiles(name string) ([]string, error) {
	var files []string
	err := fs.WalkDir(g.libraryFS(), name, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files in library entry %q", name)
	}
	return files, nil
}

// warnIfCollision logs a warning when a bare-name source resolved
// against the library ALSO exists as a directory on disk relative to
// cwd. Doesn't change behavior (library wins) — surfaces the ambiguity
// so users can disambiguate with the explicit "package:" prefix.
func (g *FileLogGenerator) warnIfCollision(name string) {
	if info, err := os.Stat(name); err == nil && info.IsDir() {
		g.logger.Warn("filegen source resolved to embedded data library, but a same-named directory exists on disk",
			zap.String("source", name),
			zap.String("hint", "use 'package:"+name+"' to force library lookup, or a relative path for disk lookup"),
		)
	}
}

// getFilesFromAutoDetect detects whether source is a file or directory and handles accordingly
func (g *FileLogGenerator) getFilesFromAutoDetect() ([]string, error) {
	// First, try to expand as a glob pattern
	globFiles, err := filepath.Glob(g.source)
	if err == nil && len(globFiles) > 0 {
		// Found glob matches, filter for both files and directories
		var files []string
		for _, f := range globFiles {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			if info.IsDir() {
				// For directories in glob results, read all files from the directory
				dirFiles, err := g.getFilesFromDirectory()
				if err == nil {
					files = append(files, dirFiles...)
				}
			} else {
				// For files in glob results, add them directly
				files = append(files, f)
			}
		}
		if len(files) > 0 {
			return files, nil
		}
	}

	// If glob didn't match files, try as a literal path
	info, err := os.Stat(g.source)
	if err != nil {
		return nil, fmt.Errorf("stat source: %w", err)
	}

	if info.IsDir() {
		return g.getFilesFromDirectory()
	}

	return g.getFilesFromFile()
}

// getFilesFromFile returns a single file
func (g *FileLogGenerator) getFilesFromFile() ([]string, error) {
	if _, err := os.Stat(g.source); err != nil {
		return nil, fmt.Errorf("check file: %w", err)
	}
	return []string{g.source}, nil
}

// getFilesFromDirectory returns all files in a directory
func (g *FileLogGenerator) getFilesFromDirectory() ([]string, error) {
	pattern := filepath.Join(g.source, "*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob directory: %w", err)
	}

	return files, nil
}

// worker reads lines from files and writes them to the output writer
// SetCountTracker sets the finite generation count tracker.
func (g *FileLogGenerator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

func (g *FileLogGenerator) worker(id int, files []string) {
	defer g.wg.Done()

	g.logger.Debug("Worker started", zap.Int("id", id))

	// Distribute files among workers
	fileIdx := id

	// Use exponential backoff with configured rate as initial interval
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0 // Never stop retrying

	// Drive the timer from this goroutine only. backoff.ExponentialBackOff is
	// not safe for concurrent use, so we never hand it to backoff.NewTicker's
	// internal goroutine; instead we own every NextBackOff/Reset call here.
	timer := time.NewTimer(backoffConfig.NextBackOff())
	defer timer.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker received stop signal", zap.Int("id", id))
			return
		case <-timer.C:
			if g.tracker != nil && !g.tracker.Acquire() {
				select {
				case <-g.stopCh:
					return
				case <-g.tracker.ResumeC():
					timer.Reset(backoffConfig.NextBackOff())
					continue
				}
			}
			if fileIdx >= len(files) {
				// Cycle back to the beginning
				fileIdx = 0
			}

			file := files[fileIdx]
			err := g.readAndWriteFile(file)
			if err != nil {
				g.logger.Error("Error reading file", zap.String("file", file), zap.Error(err))
				generator.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName)
				// On error, backoff will automatically handle retry timing
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}

			// On success, reset backoff to configured rate
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
			fileIdx += g.workers
		}
	}
}

// readAndWriteFile reads a file, selects a random non-empty line, and pushes it to the consumer
func (g *FileLogGenerator) readAndWriteFile(filename string) error {
	// Cache key namespaces library-backed paths so they can't collide
	// with disk paths in the shared LRU.
	cacheKey := filename
	if g.mode == sourceModeLibrary {
		cacheKey = "pkg://" + filename
	}

	// Check cache first
	var lines []string
	if cachedLines, found := g.cache.Get(cacheKey); found {
		lines = cachedLines
	} else {
		// Cache miss, read the file via the configured backend.
		var err error
		lines, err = g.readFileLines(filename)
		if err != nil {
			return err
		}

		// Update cache
		g.cache.Set(cacheKey, lines)
	}

	// If no non-empty lines found, return without error
	if len(lines) == 0 {
		return nil
	}

	// Select a random line
	// #nosec G404 - using weak random is acceptable for log generation, not cryptographic purposes
	randomIdx := rand.Intn(len(lines))
	selectedLine := lines[randomIdx]

	// Process timestamp directives in the line
	processedLine := g.processTimestamps(selectedLine)

	// Push as a size-1 batch with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{{
		Message: processedLine,
		Metadata: embed.LogRecordMetadata{
			Timestamp: time.Now(),
			Resource:  resource.Default("filegen", "filegen.source", g.source),
		},
	}})
	cancel()

	if err != nil {
		generator.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName)
		return fmt.Errorf("write: %w", err)
	}

	generator.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	return nil
}

// readFileLines reads all non-empty lines from a file. The backend
// (disk or library FS) is chosen by g.mode, set during getFiles.
func (g *FileLogGenerator) readFileLines(filename string) ([]string, error) {
	var rc io.ReadCloser
	if g.mode == sourceModeLibrary {
		f, err := g.libraryFS().Open(filename)
		if err != nil {
			return nil, fmt.Errorf("open library file: %w", err)
		}
		rc = f
	} else {
		// #nosec G304 - filename is controlled by the application, either from explicit config or from walking data library directory
		f, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}
		rc = f
	}
	defer rc.Close()

	// Read all non-empty lines from the file
	var lines []string
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			lines = append(lines, string(line))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return lines, nil
}

// epochDirectiveRegexp matches the Unix-epoch time directives. Their integer
// epoch values cannot be expressed as ctime layout tokens, so they are
// substituted before the line reaches the ctime package. The trailing \b keeps
// a valid unit followed by more letters (e.g. %EPOCH_SOMETHING) from being
// partially matched.
var epochDirectiveRegexp = regexp.MustCompile(`%EPOCH_(NS|US|MS|S)\b`)

// replaceEpochDirectives substitutes the %EPOCH_* directives with the integer
// Unix epoch of t in the requested unit, emitted as a plain integer (no
// fractional part, no separators). Other directives, including the sub-second
// %s token, are left untouched for the ctime formatter downstream.
func replaceEpochDirectives(line string, t time.Time) string {
	return epochDirectiveRegexp.ReplaceAllStringFunc(line, func(directive string) string {
		switch directive {
		case "%EPOCH_NS":
			return strconv.FormatInt(t.UnixNano(), 10)
		case "%EPOCH_US":
			return strconv.FormatInt(t.UnixMicro(), 10)
		case "%EPOCH_MS":
			return strconv.FormatInt(t.UnixMilli(), 10)
		default: // %EPOCH_S
			return strconv.FormatInt(t.Unix(), 10)
		}
	})
}

// processTimestamps replaces timestamp directives in the line with actual formatted timestamps
func (g *FileLogGenerator) processTimestamps(line string) string {
	now := time.Now()

	// Short-circuit the Unix-epoch directives before the ctime passes, since
	// their integer values cannot be expressed as ctime layout tokens.
	line = replaceEpochDirectives(line, now)

	// Process common multi-directive patterns first for performance
	// These are the most frequently used patterns that should be optimized
	commonPatterns := []struct {
		pattern   string
		formatter func(time.Time) string
	}{
		{"%Y-%m-%dT%H:%M:%S.%3NZ", func(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }},
		{"%Y-%m-%dT%H:%M:%SZ", func(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }},
		{"%Y-%m-%dT%H:%M:%S", func(t time.Time) string { return t.Format("2006-01-02T15:04:05") }},
		{"%Y/%m/%d %H:%M:%S", func(t time.Time) string { return t.Format("2006/01/02 15:04:05") }},
		{"%b %d %H:%M:%S", func(t time.Time) string { return t.Format("Jan 02 15:04:05") }},
		{"%b %e %T", func(t time.Time) string { return t.Format("Jan _2 15:04:05") }},
	}

	result := line

	// First, process common patterns
	for _, pattern := range commonPatterns {
		re := regexp.MustCompile(regexp.QuoteMeta(pattern.pattern))
		if re.MatchString(result) {
			result = re.ReplaceAllString(result, pattern.formatter(now))
		}
	}

	// Then process all individual ctime directives using the ctime package
	// This handles all remaining directives according to the ctime standard
	directivePattern := regexp.MustCompile(`%[YymdoqbhBdeagAHIlpPMSLfsZzwxFTXrRnct%]`)
	result = directivePattern.ReplaceAllStringFunc(result, func(directive string) string {
		// For each directive, use the ctime package to format it
		// We use ctime.Format to handle the directive properly
		formatted, err := ctime.Format(directive, now)
		if err != nil {
			// If formatting fails, return the original directive unchanged
			g.logger.Debug("Failed to format ctime directive", zap.String("directive", directive), zap.Error(err))
			return directive
		}
		return formatted
	})

	return result
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *FileLogGenerator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
