package file

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// Mode defines the file reading mode
type Mode string

const (
	ModeFile      Mode = "file"
	ModeDirectory Mode = "directory"
	ModePackage   Mode = "package"
)

// FileLogGenerator generates log data by reading from files
type FileLogGenerator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration
	mode    Mode
	source  string // file path or directory path or package name
	pattern string // glob pattern for directory mode
	stopCh  chan struct{}
	wg      sync.WaitGroup
	meter   metric.Meter

	// Metrics
	logsGenerated metric.Int64Counter
	activeWorkers metric.Int64Gauge
	writeErrors   metric.Int64Counter
}

// New creates a new File log generator
func New(logger *zap.Logger, workers int, rate time.Duration, mode Mode, source, pattern string) (*FileLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	if rate <= 0 {
		return nil, fmt.Errorf("rate must be positive, got %v", rate)
	}

	// Validate mode
	switch mode {
	case ModeFile, ModeDirectory, ModePackage:
	default:
		return nil, fmt.Errorf("invalid mode %q, must be one of: file, directory, package", mode)
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

	return &FileLogGenerator{
		logger:        logger,
		workers:       workers,
		rate:          rate,
		mode:          mode,
		source:        source,
		pattern:       pattern,
		stopCh:        make(chan struct{}),
		meter:         meter,
		logsGenerated: logsGenerated,
		activeWorkers: activeWorkers,
		writeErrors:   writeErrors,
	}, nil
}

// Start starts the File log generator
func (g *FileLogGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting File log generator",
		zap.String("mode", string(g.mode)),
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

// getFiles returns a list of files to read based on the mode
func (g *FileLogGenerator) getFiles() ([]string, error) {
	switch g.mode {
	case ModeFile:
		return g.getFilesFromFile()
	case ModeDirectory:
		return g.getFilesFromDirectory()
	case ModePackage:
		return g.getFilesFromPackage()
	default:
		return nil, fmt.Errorf("unknown mode: %s", g.mode)
	}
}

// getFilesFromFile returns a single file
func (g *FileLogGenerator) getFilesFromFile() ([]string, error) {
	if _, err := os.Stat(g.source); err != nil {
		return nil, fmt.Errorf("check file: %w", err)
	}
	return []string{g.source}, nil
}

// getFilesFromDirectory returns all files matching the pattern in a directory
func (g *FileLogGenerator) getFilesFromDirectory() ([]string, error) {
	pattern := g.pattern
	if pattern == "" {
		pattern = "*"
	}

	var files []string
	err := filepath.Walk(g.source, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Match against pattern
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return err
		}
		if matched {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return files, nil
}

// getFilesFromPackage returns all files from the built-in data library
func (g *FileLogGenerator) getFilesFromPackage() ([]string, error) {
	// Data library files are in data_library/<packagename>/
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
		return nil, fmt.Errorf("walk data library: %w", err)
	}

	return files, nil
}

// worker reads lines from files and writes them to the output writer
func (g *FileLogGenerator) worker(id int, writer output.Writer, files []string) {
	defer g.wg.Done()

	g.logger.Debug("Worker started", zap.Int("id", id))

	// Distribute files among workers
	fileIdx := id
	backoff := backoff.NewExponentialBackOff()
	backoff.InitialInterval = g.rate

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker received stop signal", zap.Int("id", id))
			return
		default:
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
		}

		fileIdx += g.workers

		// Apply backoff for rate limiting
		ticker := time.NewTicker(backoff.NextBackOff())
		select {
		case <-g.stopCh:
			ticker.Stop()
			return
		case <-ticker.C:
			ticker.Stop()
		}
	}
}

// readAndWriteFile reads a file line by line and writes each line to the writer
func (g *FileLogGenerator) readAndWriteFile(filename string, writer output.Writer) error {
	// #nosec G304 - filename is controlled by the application, either from explicit config or from walking pre-packaged data library
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		select {
		case <-g.stopCh:
			return nil
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Write with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := writer.Write(ctx, output.LogRecord{
			Message: string(line),
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
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}
