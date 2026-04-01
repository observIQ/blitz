package filegen

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/generator/ctime"
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

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
	logger  *zap.Logger
	workers int
	rate    time.Duration
	source  string // file path or directory path or glob pattern
	stopCh  chan struct{}
	tracker *count.Tracker
	wg      sync.WaitGroup
	meter   metric.Meter

	// Metrics
	logsGenerated metric.Int64Counter
	activeWorkers metric.Int64Gauge
	writeErrors   metric.Int64Counter

	// File cache
	cache *Cache
}

// New creates a new File log generator
func New(logger *zap.Logger, workers int, rate time.Duration, source string, cacheEnabled bool, cacheTTL time.Duration) (*FileLogGenerator, error) {
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

	meter := otel.Meter("blitz-generator")

	logsGenerated, err := meter.Int64Counter(
		"blitz.generator.logs.generated",
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	activeWorkers, err := meter.Int64Gauge(
		"blitz.generator.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	writeErrors, err := meter.Int64Counter(
		"blitz.generator.write.errors",
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	cache, err := NewCache(cacheEnabled, cacheTTL, 1000) // 1000 is max size for LRU
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	return &FileLogGenerator{
		logger:        logger,
		workers:       workers,
		rate:          rate,
		source:        source,
		stopCh:        make(chan struct{}),
		meter:         meter,
		logsGenerated: logsGenerated,
		activeWorkers: activeWorkers,
		writeErrors:   writeErrors,
		cache:         cache,
	}, nil
}

// Start starts the File log generator
func (g *FileLogGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting File log generator",
		zap.String("source", g.source),
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate))

	// Record initial active workers count
	g.activeWorkers.Record(context.Background(), int64(g.workers),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_file"),
			),
		),
	)

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
		go g.worker(i, writer, files)
	}

	return nil
}

// Stop stops the File log generator
func (g *FileLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping File log generator")

	g.activeWorkers.Record(ctx, 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_file"),
			),
		),
	)

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

// getFiles returns a list of files to read, auto-detecting whether source is a file or directory
func (g *FileLogGenerator) getFiles() ([]string, error) {
	return g.getFilesFromAutoDetect()
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

// getFilesFromPackage returns all files from the data library directory
func (g *FileLogGenerator) getFilesFromPackage() ([]string, error) {
	// Data library files are in data_library/<packagename>/ (relative path)
	packagesDir := filepath.Join("data_library", g.source)

	var files []string
	err := filepath.Walk(packagesDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk package directory: %w", err)
	}

	return files, nil
}

// worker reads lines from files and writes them to the output writer
// SetCountTracker sets the finite generation count tracker.
func (g *FileLogGenerator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

func (g *FileLogGenerator) worker(id int, writer output.Writer, files []string) {
	defer g.wg.Done()

	g.logger.Debug("Worker started", zap.Int("id", id))

	// Distribute files among workers
	fileIdx := id

	// Use exponential backoff with configured rate as initial interval
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0 // Never stop retrying

	backoffTicker := backoff.NewTicker(backoffConfig)
	defer backoffTicker.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker received stop signal", zap.Int("id", id))
			return
		case <-backoffTicker.C:
			if g.tracker != nil && !g.tracker.Acquire() {
				select {
				case <-g.stopCh:
					return
				case <-g.tracker.ResumeC():
					continue
				}
			}
			if fileIdx >= len(files) {
				// Cycle back to the beginning
				fileIdx = 0
			}

			file := files[fileIdx]
			err := g.readAndWriteFile(file, writer)
			if err != nil {
				g.logger.Error("Error reading file", zap.String("file", file), zap.Error(err))
				g.writeErrors.Add(context.Background(), 1,
					metric.WithAttributeSet(
						attribute.NewSet(
							attribute.String("component", "generator_file"),
						),
					),
				)
				// On error, backoff will automatically handle retry timing
				continue
			}

			// On success, reset backoff to configured rate
			backoffConfig.Reset()
			fileIdx += g.workers
		}
	}
}

// readAndWriteFile reads a file, selects a random non-empty line, and writes it to the writer
func (g *FileLogGenerator) readAndWriteFile(filename string, writer output.Writer) error {
	// Check cache first
	var lines []string
	if cachedLines, found := g.cache.Get(filename); found {
		lines = cachedLines
	} else {
		// Cache miss, read from disk
		var err error
		lines, err = g.readFileLines(filename)
		if err != nil {
			return err
		}

		// Update cache
		g.cache.Set(filename, lines)
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

	// Write with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := writer.Write(ctx, output.LogRecord{
		Message: processedLine,
		Metadata: output.LogRecordMetadata{
			Timestamp: time.Now(),
		},
	})
	cancel()

	if err != nil {
		g.writeErrors.Add(context.Background(), 1,
			metric.WithAttributeSet(
				attribute.NewSet(
					attribute.String("component", "generator_file"),
				),
			),
		)
		return fmt.Errorf("write: %w", err)
	}

	g.logsGenerated.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_file"),
			),
		),
	)

	return nil
}

// readFileLines reads all non-empty lines from a file
func (g *FileLogGenerator) readFileLines(filename string) ([]string, error) {
	// #nosec G304 - filename is controlled by the application, either from explicit config or from walking data library directory
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Read all non-empty lines from the file
	var lines []string
	scanner := bufio.NewScanner(file)
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

// processTimestamps replaces timestamp directives in the line with actual formatted timestamps
func (g *FileLogGenerator) processTimestamps(line string) string {
	now := time.Now()

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
