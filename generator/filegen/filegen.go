package filegen

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/generator/ctime"
	"github.com/observiq/blitz/internal/generator/pacing"
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
	static      *resource.StaticResources
	dataLibrary fs.FS // optional; nil falls back to ./data_library on disk for "package:" / bare-name sources
	stopCh      chan struct{}
	tracker     *count.Tracker
	wg          sync.WaitGroup

	// resolved backend; set in Start
	mode sourceMode

	// replay emits every line once, in order, instead of random picks.
	// replayLines is resolved once, before the worker starts; the worker only
	// reads it.
	replay      bool
	replayLines []string

	// seed drives the random mode's line picks; negative is resolved to a
	// random seed at Start, and worker N uses seed+N.
	seed int64

	// File cache
	cache *Cache

	metrics *generator.Metrics
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
func New(logger *zap.Logger, workers int, rate time.Duration, source string, cacheEnabled bool, cacheTTL time.Duration, consumer embed.LogConsumer, dataLibrary fs.FS, tel embed.TelemetrySettings, opts ...generator.Option) (*FileLogGenerator, error) {
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

	metrics, err := generator.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build generator metrics: %w", err)
	}

	return &FileLogGenerator{
		logger:      logger,
		workers:     workers,
		rate:        rate,
		source:      source,
		consumer:    consumer,
		static:      resource.FromIdentity(generator.NewOptions(opts...).Identity, "filegen", "filegen.source", source),
		dataLibrary: dataLibrary,
		stopCh:      make(chan struct{}),
		cache:       cache,
		metrics:     metrics,
		seed:        -1,
	}, nil
}

// SetSeed sets the random mode's RNG seed per the docs/datagen.md contract:
// negative randomizes (the resolved seed is logged at Start), 0 or positive
// is deterministic. Call it before Start.
func (g *FileLogGenerator) SetSeed(seed int64) {
	g.seed = seed
}

// SetReplay switches the generator to replay mode: every non-empty line of
// the source is emitted once, in order (files in lexical order, each top to
// bottom). Without a count tracker the generator then stops; with one it
// waits for a reset and replays from the first line. Replay is one ordered
// stream, so it requires a single worker. Call it before Start.
func (g *FileLogGenerator) SetReplay() error {
	if g.workers != 1 {
		return fmt.Errorf("replay mode requires exactly 1 worker, got %d workers", g.workers)
	}
	g.replay = true
	return nil
}

// ReplayCount returns the number of lines a replay emits, so a caller can
// bound the run to it, and 0 in random mode. Call it before Start.
func (g *FileLogGenerator) ReplayCount() (int, error) {
	if !g.replay {
		return 0, nil
	}
	lines, err := g.loadReplay()
	return len(lines), err
}

// loadReplay resolves the source and reads every file's lines, in order.
// A successful read is kept, so later calls return it without reading again.
func (g *FileLogGenerator) loadReplay() ([]string, error) {
	if g.replayLines != nil {
		return g.replayLines, nil
	}
	files, err := g.getFiles()
	if err != nil {
		return nil, fmt.Errorf("get files: %w", err)
	}
	lines := []string{}
	for _, f := range files {
		fl, err := g.readFileLines(f)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fl...)
	}
	g.replayLines = lines
	return lines, nil
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

	if g.replay {
		lines, err := g.loadReplay()
		if err != nil {
			return err
		}
		if len(lines) == 0 {
			return fmt.Errorf("no lines to replay in source %q", g.source)
		}
		g.logger.Info("Replaying source", zap.Int("lines", len(lines)))
		g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 1, componentName)
		g.wg.Add(1)
		go g.replayWorker(lines) // #nosec G118 -- the worker is bounded by Stop() and the WaitGroup, not the Start context
		return nil
	}

	if g.seed < 0 {
		g.seed = time.Now().UnixNano()
	}
	g.logger.Info("File generator seed", zap.Int64("seed", g.seed))

	// Get list of files to read
	files, err := g.getFiles()
	if err != nil {
		return fmt.Errorf("get files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found to read")
	}
	if !g.anyLines(files) {
		return fmt.Errorf("no non-empty lines in source %q", g.source)
	}

	g.logger.Info("Found files to read", zap.Int("count", len(files)))

	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)
	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, files) // #nosec G118 -- workers are bounded by Stop() and the WaitGroup, not the Start context
	}

	return nil
}

// anyLines reports whether any file has a non-empty line, reading only until
// it finds one. A worker skips a file with none, so without one anywhere it
// would loop on the source and send nothing. An unreadable file is left for
// the workers to report.
func (g *FileLogGenerator) anyLines(files []string) bool {
	for _, f := range files {
		if lines, err := g.readFileLines(f); err != nil || len(lines) > 0 {
			return true
		}
	}
	return false
}

// Stop stops the File log generator
func (g *FileLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping File log generator")

	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, componentName)

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
// A same-named directory on disk wins over the library. When the bare name
// falls back to the library anyway, because that directory could not be
// read, a warning says why the directory was not used.
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
// over the embedded one, disk winning per path. Either layer
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

// warnIfCollision logs a warning when a bare-name source fell back to the
// library while a directory of that name exists on disk, which happens only
// when the directory could not be read.
func (g *FileLogGenerator) warnIfCollision(name string) {
	if info, err := os.Stat(name); err == nil && info.IsDir() {
		g.logger.Warn("filegen source resolved to embedded data library, but a same-named directory exists on disk",
			zap.String("source", name),
			zap.String("hint", "the directory could not be read; fix its permissions to use it, or use 'package:"+name+"' to choose the library"),
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
				// Replay promises every line, so a match it cannot stat
				// fails the run; random mode skips it.
				if g.replay {
					return nil, fmt.Errorf("stat %q: %w", f, err)
				}
				g.logger.Warn("Skipping a matched path that cannot be read", zap.String("path", f), zap.Error(err))
				continue
			}
			if info.IsDir() {
				// For directories in glob results, read all files from the directory
				dirFiles, err := filesInDirectory(f)
				if err != nil {
					// Likewise for a matched directory it cannot list.
					if g.replay {
						return nil, err
					}
					g.logger.Warn("Skipping a matched directory that cannot be read", zap.String("dir", f), zap.Error(err))
					continue
				}
				files = append(files, dirFiles...)
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
		return filesInDirectory(g.source)
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

// filesInDirectory returns the files directly in dir, in lexical order,
// skipping subdirectories and symlinks to directories.
func filesInDirectory(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	var files []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			continue
		}
		if e.Type()&fs.ModeSymlink != 0 {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				continue
			}
		}
		files = append(files, path)
	}
	return files, nil
}

// SetCountTracker sets the finite generation count tracker.
func (g *FileLogGenerator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

// worker sends a random line from its share of the files on each tick.
func (g *FileLogGenerator) worker(id int, files []string) {
	defer g.wg.Done()

	g.logger.Debug("Worker started", zap.Int("id", id))

	// #nosec G404 -- seeded for the determinism contract, not cryptographic use
	rng := rand.New(rand.NewSource(g.seed + int64(id)))

	// Distribute files among workers
	fileIdx := id

	// Use exponential backoff with configured rate as initial interval
	backoffConfig := pacing.NewWriteBackoff(g.rate)

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
			// Wrap rather than restart at file 0, so the workers together
			// keep covering every file.
			fileIdx %= len(files)

			file := files[fileIdx]
			lines, err := g.fileLines(file)
			if err != nil {
				if errors.Is(err, errNoLines) {
					// Nothing failed, so keep the rate.
					backoffConfig.Reset()
				} else {
					g.logger.Error("Error reading file", zap.String("file", file), zap.Error(err))
				}
				timer.Reset(backoffConfig.NextBackOff())
				// Step to the very next file, not the next in this worker's
				// share, which may hold nothing readable.
				fileIdx++
				continue
			}

			permit, wait, ok := g.tracker.Acquire()
			if !ok {
				if !pacing.AwaitWake(g.stopCh, wait, backoffConfig, timer) {
					return
				}
				continue
			}
			if err := g.emit(count.WithPermit(context.Background(), permit), lines[rng.Intn(len(lines))]); err != nil {
				g.logger.Error("Error sending from file", zap.String("file", file), zap.Error(err))
				permit.Release()
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}

			// On success, reset backoff to configured rate
			permit.Commit()
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
			fileIdx += g.workers
		}
	}
}

// replayWorker emits lines once, in order. At the end of the input, or when
// the count tracker runs out, it waits for a tracker reset and replays from
// the first line; without a tracker it stops.
func (g *FileLogGenerator) replayWorker(lines []string) {
	defer g.wg.Done()

	backoffConfig := pacing.NewWriteBackoff(g.rate)

	timer := time.NewTimer(backoffConfig.NextBackOff())
	defer timer.Stop()

	idx := 0
	// retries holds lines whose send failed, retried in order after the pass.
	var retries []replayRetry
	// cycle is the tracker's cycle channel for the pass in progress. Acquire
	// reports the current one with every grant, so a changed channel means a
	// reset (SIGUSR1) landed and the refilled count starts a fresh pass.
	var cycle <-chan struct{}

	for {
		select {
		case <-g.stopCh:
			return
		case <-timer.C:
			if idx >= len(lines) && len(retries) == 0 {
				if g.tracker == nil {
					g.logger.Info("Replay complete", zap.Int("lines", len(lines)))
					g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 0, componentName)
					return
				}
				// End of input, the normal end of a counted replay: wait for a
				// reset, then replay from the first line. A tracker larger
				// than the line count (the CLI bounds it; a caller of
				// SetCountTracker may not) leaves its extra permits undrawn.
				if !pacing.AwaitWake(g.stopCh, cycle, backoffConfig, timer) {
					return
				}
				idx = 0
				continue
			}
			permit, wait, ok := g.tracker.Acquire()
			if !ok {
				if !pacing.AwaitWake(g.stopCh, wait, backoffConfig, timer) {
					return
				}
				continue
			}
			if wait != cycle {
				idx = 0
				retries = nil
				cycle = wait
			}
			// The pass sends each line in turn, then the retry queue.
			cur := replayRetry{line: idx}
			retrying := idx >= len(lines)
			if retrying {
				cur = retries[0]
				retries = retries[1:]
				g.logger.Warn("Retrying a replay line",
					zap.Int("index", cur.line), zap.Int("retry", cur.tries+1), zap.Int("of", replayRetries))
			} else {
				idx++
			}
			err := g.emit(count.WithPermit(context.Background(), permit), lines[cur.line])
			if err != nil {
				permit.Release()
			} else {
				permit.Commit()
			}
			// Wait for the destination's result before the next send, so
			// lines arrive in order and a failed line is queued only once
			// every output has settled it.
			select {
			case <-g.stopCh:
				return
			case <-permit.Settled():
			}
			// A failed send counts as undelivered even with no tracker, whose
			// zero permit always reports delivered.
			if err != nil || !permit.Delivered() {
				if err == nil {
					err = errNotDelivered
				}
				if retrying {
					cur.tries++
				}
				if cur.tries >= replayRetries {
					g.logger.Error("Dropping a replay line after its retries failed",
						zap.Int("index", cur.line), zap.Int("retries", cur.tries), zap.Error(err))
					permit.Forgo()
				} else {
					g.logger.Error("Replayed line was not delivered, queued for retry",
						zap.Int("index", cur.line), zap.Error(err))
					retries = append(retries, cur)
				}
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
		}
	}
}

// replayRetries is how many times a failed replay line is retried before it
// is dropped.
const replayRetries = 3

// replayRetry is a replay line waiting for another send, and the retries it
// has had.
type replayRetry struct {
	line  int
	tries int
}

// errNotDelivered reports a line an output accepted and then failed to
// deliver; that output logs the cause.
var errNotDelivered = errors.New("an output did not deliver the line")

// errNoLines reports a file with no non-empty lines to send.
var errNoLines = errors.New("file has no non-empty lines")

// fileLines returns a file's non-empty lines, through the cache. It is read
// before a permit is taken, so a file with nothing to send never takes one.
func (g *FileLogGenerator) fileLines(filename string) ([]string, error) {
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
			// A failed read never reaches emit, which counts write failures, so
			// it is counted here.
			g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName)
			return nil, err
		}

		// Update cache
		g.cache.Set(cacheKey, lines)
	}

	if len(lines) == 0 {
		return nil, errNoLines
	}
	return lines, nil
}

// emit processes the line's timestamp directives and pushes it to the
// consumer as a size-1 batch.
func (g *FileLogGenerator) emit(ctx context.Context, line string) error {
	processedLine := g.processTimestamps(line)

	// Push as a size-1 batch with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{{
		Message: processedLine,
		Metadata: embed.LogRecordMetadata{
			Timestamp: time.Now(),
			Resource:  g.static.Record(),
		},
	}})
	cancel()

	if err != nil {
		g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName)
		return err
	}

	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

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
	// Lift the default 64 KiB token limit so long lines are read whole.
	scanner.Buffer(nil, math.MaxInt)
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

// commonTimestampPatterns are the most frequent multi-directive patterns,
// replaced whole before the per-directive pass.
var commonTimestampPatterns = []struct {
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

// directivePattern matches one ctime directive.
var directivePattern = regexp.MustCompile(`%[YymdoqbhBdeagAHIlpPMSLfsZzwxFTXrRnct%]`)

// processTimestamps replaces timestamp directives in the line with actual formatted timestamps
func (g *FileLogGenerator) processTimestamps(line string) string {
	now := time.Now()
	if !strings.Contains(line, "%%") {
		return g.renderDirectives(line, now)
	}
	// %% is a literal %, so render the text between escapes on its own; no
	// pass, the epoch and whole-pattern ones included, can match across one.
	parts := strings.Split(line, "%%")
	for i, part := range parts {
		parts[i] = g.renderDirectives(part, now)
	}
	return strings.Join(parts, "%")
}

// renderDirectives renders the timestamp directives in text that holds no %%.
func (g *FileLogGenerator) renderDirectives(line string, now time.Time) string {
	// Short-circuit the Unix-epoch directives before the ctime passes, since
	// their integer values cannot be expressed as ctime layout tokens.
	line = replaceEpochDirectives(line, now)

	result := line

	// First, process common patterns
	for _, pattern := range commonTimestampPatterns {
		if strings.Contains(result, pattern.pattern) {
			result = strings.ReplaceAll(result, pattern.pattern, pattern.formatter(now))
		}
	}

	// Then process all individual ctime directives using the ctime package
	// This handles all remaining directives according to the ctime standard
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
