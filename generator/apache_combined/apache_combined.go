package apache_combined

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/internal/useragent"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const componentName = "apache-combined"

// apacheCombinedLogData represents the data needed to generate an Apache Combined Log Format entry
type apacheCombinedLogData struct {
	remoteHost string
	identity   string
	userID     string
	timestamp  time.Time
	request    string
	statusCode int
	size       int
	referer    string
	userAgent  string
	severity   string
}

// ApacheCombinedLogGenerator generates Apache Combined Log Format log data
type ApacheCombinedLogGenerator struct {
	embed.ProducerMarker

	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
}

// New creates a new Apache Combined log generator
func New(logger *zap.Logger, workers int, rate time.Duration) (*ApacheCombinedLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	return &ApacheCombinedLogGenerator{
		logger:  logger,
		workers: workers,
		rate:    rate,
		stopCh:  make(chan struct{}),
	}, nil
}

// Start starts the Apache Combined log generator and writes data using the
// provided generator writer.
func (g *ApacheCombinedLogGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting Apache Combined log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the Apache Combined log generator and waits for all workers to finish.
func (g *ApacheCombinedLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Apache Combined log generator")

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("Apache Combined log generator stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SetCountTracker sets the finite generation count tracker.
func (g *ApacheCombinedLogGenerator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

// worker is the main worker loop that generates and writes logs
func (g *ApacheCombinedLogGenerator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()

	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 1, componentName)
	defer generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 0, componentName)

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0 // Never stop retrying

	backoffTicker := backoff.NewTicker(backoffConfig)
	defer backoffTicker.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker stopping", zap.Int("worker_id", workerID))
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
			err := g.generateAndWriteLog(writer, workerID)
			if err != nil {
				g.logger.Error("Failed to write log",
					zap.Int("worker_id", workerID),
					zap.Error(err))
				continue
			}
			backoffConfig.Reset()
		}
	}
}

// generateAndWriteLog generates a random log and writes it
func (g *ApacheCombinedLogGenerator) generateAndWriteLog(writer output.Writer, workerID int) error {
	// Generate Apache Combined log data
	logData, err := g.generateApacheCombinedLogData()
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("generate Apache Combined log data: %w", err)
	}

	// Format log data as Apache Combined Log Format
	logRecord, err := formatAsApacheCombined(logData)
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("format log as Apache Combined: %w", err)
	}

	// Record logs generated counter
	generator.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	// Write the data with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writer.Write(ctx, logRecord); err != nil {
		// Classify error type
		errorType := "unknown"
		if ctx.Err() == context.DeadlineExceeded {
			errorType = "timeout"
		}
		g.recordWriteError(errorType, err)
		return err
	}

	return nil
}

// generateApacheCombinedLogData generates random Apache Combined log data
func (g *ApacheCombinedLogGenerator) generateApacheCombinedLogData() (*apacheCombinedLogData, error) {
	// Use fast random generator with gosec nosec comment
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	data := &apacheCombinedLogData{
		timestamp: time.Now(),
	}

	// Generate remote host IP address
	data.remoteHost = datagen.RandomIPv4(r)

	// Identity is typically "-" in CLF
	data.identity = "-"

	// Generate request
	data.request = generateRequest(r)

	// Generate status code and severity
	data.statusCode, data.severity = generateStatusAndSeverity(r)

	// Generate response size (100 bytes to 10MB)
	data.size = r.Intn(10000000) + 100 // #nosec G404

	// User ID is typically "-" in CLF
	data.userID = "-"

	// Generate referer
	data.referer = generateReferer(r)

	// Generate user agent
	data.userAgent = useragent.RandomUserAgent(r)

	return data, nil
}

// generateRequest generates a random HTTP request string
func generateRequest(r *rand.Rand) string {
	method := datagen.Methods.Random(r)
	path := datagen.APIPaths.Random(r)
	protocol := datagen.Protocols.Random(r)
	return fmt.Sprintf("%s %s %s", method, path, protocol)
}

// generateStatusAndSeverity generates a random HTTP status code and corresponding severity
func generateStatusAndSeverity(r *rand.Rand) (int, string) {
	status := datagen.RandomStatusCode(r)
	switch {
	case status >= 500:
		return status, "ERROR"
	case status >= 400:
		return status, "WARN"
	default:
		return status, "INFO"
	}
}

// generateReferer generates a random referer URL
func generateReferer(r *rand.Rand) string {
	// Sometimes no referer (direct access)
	if r.Float64() < 0.3 { // #nosec G404
		return "-"
	}

	domain := datagen.RefererDomains.Random(r)
	path := datagen.APIPaths.Random(r)
	return fmt.Sprintf("https://%s%s", domain, path)
}

// formatAsApacheCombined converts apacheCombinedLogData to Apache Combined Log Format
// Format: remotehost rfc931 authuser [date] "request" status bytes "referer" "user-agent"
// Example: 127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326 "http://www.example.com/start.html" "Mozilla/4.08 [en] (Win98; I ;Nav)"
func formatAsApacheCombined(data *apacheCombinedLogData) (output.LogRecord, error) {
	// Format timestamp as [dd/MMM/yyyy:HH:mm:ss -TZ]
	// Use local timezone offset
	loc := time.Now().Location()
	localTime := data.timestamp.In(loc)
	_, offset := localTime.Zone()
	offsetHours := offset / 3600
	offsetMins := (offset % 3600) / 60
	offsetSign := "+"
	if offsetHours < 0 {
		offsetSign = "-"
		offsetHours = -offsetHours
		offsetMins = -offsetMins
	}
	timestampStr := localTime.Format(fmt.Sprintf("02/Jan/2006:15:04:05 %s%02d%02d", offsetSign, offsetHours, offsetMins))

	// Build Combined Log Format line
	combinedLine := fmt.Sprintf(`%s %s %s [%s] "%s" %d %d "%s" "%s"`,
		data.remoteHost,
		data.identity,
		data.userID,
		timestampStr,
		data.request,
		data.statusCode,
		data.size,
		data.referer,
		data.userAgent,
	)

	return output.LogRecord{
		Message: combinedLine,
		ParseFunc: func(message string) (map[string]any, error) {
			// Basic parsing - split by spaces but handle quoted fields
			parts := strings.Fields(message)
			if len(parts) < 9 {
				return nil, fmt.Errorf("invalid Apache Combined log format: expected at least 9 fields, got %d", len(parts))
			}

			parsed := make(map[string]any)
			parsed["remote_host"] = parts[0]
			parsed["identity"] = parts[1]
			parsed["user_id"] = parts[2]
			// Timestamp is in brackets, need to extract it
			if len(parts) > 3 {
				parsed["timestamp"] = strings.Trim(parts[3], "[]")
			}
			// Request is quoted, need to extract it
			if len(parts) > 4 {
				parsed["request"] = strings.Trim(parts[4], `"`)
			}
			if len(parts) > 5 {
				parsed["status_code"] = parts[5]
			}
			if len(parts) > 6 {
				parsed["size"] = parts[6]
			}
			if len(parts) > 7 {
				parsed["referer"] = strings.Trim(parts[7], `"`)
			}
			if len(parts) > 8 {
				parsed["user_agent"] = strings.Trim(parts[8], `"`)
			}

			return parsed, nil
		},
		Metadata: output.LogRecordMetadata{
			Timestamp: data.timestamp,
			Severity:  data.severity,
		},
	}, nil
}

// recordWriteError records a write error metric
func (g *ApacheCombinedLogGenerator) recordWriteError(errorType string, err error) {
	generator.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
	)
	g.logger.Debug("Recorded write error",
		zap.String("error_type", errorType),
		zap.Error(err),
	)
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *ApacheCombinedLogGenerator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
