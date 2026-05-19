package apache

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/generator/security"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const componentName = "apache"

// apacheLogData represents the data needed to generate an Apache CLF log entry
type apacheLogData struct {
	remoteHost string
	identity   string
	userID     string
	timestamp  time.Time
	request    string
	statusCode int
	size       int
	severity   string
}

// ApacheLogGenerator generates Apache Common Log Format (CLF) log data
type ApacheLogGenerator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
}

// New creates a new Apache log generator
func New(logger *zap.Logger, workers int, rate time.Duration) (*ApacheLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	return &ApacheLogGenerator{
		logger:  logger,
		workers: workers,
		rate:    rate,
		stopCh:  make(chan struct{}),
	}, nil
}

// Start starts the Apache log generator and writes data using the
// provided generator writer.
func (g *ApacheLogGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting Apache log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate))

	// Record initial active workers count
	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the Apache log generator.
// This function expects to be called exactly once.
func (g *ApacheLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Apache log generator")

	// Record zero active workers
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
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

// SetCountTracker sets the finite generation count tracker.
func (g *ApacheLogGenerator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

// worker runs a single worker goroutine
func (g *ApacheLogGenerator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()

	g.logger.Debug("Starting worker", zap.Int("worker_id", workerID))

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
func (g *ApacheLogGenerator) generateAndWriteLog(writer output.Writer, workerID int) error {
	// Generate Apache log data
	logData, err := g.generateApacheLogData()
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("generate Apache log data: %w", err)
	}

	// Format log data as Apache CLF
	logRecord, err := formatAsApacheCLF(logData)
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("format log as Apache CLF: %w", err)
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

// generateApacheLogData generates random Apache log data
func (g *ApacheLogGenerator) generateApacheLogData() (*apacheLogData, error) {
	// Use fast random generator with gosec nosec comment
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	data := &apacheLogData{
		timestamp: time.Now(),
	}

	// Generate remote host IP address
	data.remoteHost = generateRandomIP(r)

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

	return data, nil
}

// generateRandomIP generates a random IP address
func generateRandomIP(r *rand.Rand) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		r.Intn(256), // #nosec G404
		r.Intn(256), // #nosec G404
		r.Intn(256), // #nosec G404
		r.Intn(256)) // #nosec G404
}

// generateRequest generates a random HTTP request string
func generateRequest(r *rand.Rand) string {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	method := methods[r.Intn(len(methods))] // #nosec G404

	// Normal paths
	normalPaths := []string{
		"/api/v1/users",
		"/api/v1/orders",
		"/health",
		"/status",
		"/api/v2/data",
		"/index.html",
		"/api/v1/auth",
		"/api/v1/payments",
		"/api/v1/transactions",
		"/api/v1/accounts",
		"/api/v1/products",
		"/api/v1/inventory",
		"/api/v1/customers",
		"/api/v1/loans",
		"/api/v1/transfers",
		"/api/v1/verification",
	}

	// 20% chance of generating a security-focused path
	var path string
	if r.Float64() < 0.20 { // #nosec G404
		path = security.RandomAttackPath(r)
	} else {
		path = normalPaths[r.Intn(len(normalPaths))] // #nosec G404
	}

	protocols := []string{"HTTP/1.0", "HTTP/1.1", "HTTP/2.0"}
	protocol := protocols[r.Intn(len(protocols))] // #nosec G404

	return fmt.Sprintf("%s %s %s", method, path, protocol)
}

// generateStatusAndSeverity generates a random HTTP status code and corresponding severity
func generateStatusAndSeverity(r *rand.Rand) (int, string) {
	// Weight status codes to be more realistic (mostly 2xx, some 4xx, few 5xx)
	roll := r.Float64() // #nosec G404

	switch {
	case roll < 0.85: // 85% success
		statusCodes := []int{200, 201, 204}
		status := statusCodes[r.Intn(len(statusCodes))] // #nosec G404
		return status, "INFO"
	case roll < 0.95: // 10% client errors
		statusCodes := []int{400, 401, 403, 404, 429}
		status := statusCodes[r.Intn(len(statusCodes))] // #nosec G404
		return status, "WARN"
	default: // 5% server errors
		statusCodes := []int{500, 502, 503, 504}
		status := statusCodes[r.Intn(len(statusCodes))] // #nosec G404
		return status, "ERROR"
	}
}

// formatAsApacheCLF converts apacheLogData to Apache Common Log Format
// Format: remotehost rfc931 authuser [date] "request" status bytes
// Example: 127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326
func formatAsApacheCLF(data *apacheLogData) (output.LogRecord, error) {
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

	// Build CLF line
	clfLine := fmt.Sprintf(`%s %s %s [%s] "%s" %d %d`,
		data.remoteHost,
		data.identity,
		data.userID,
		timestampStr,
		data.request,
		data.statusCode,
		data.size,
	)

	return output.LogRecord{
		Message: clfLine,
		ParseFunc: func(message string) (map[string]any, error) {
			return parseApacheCLF(message)
		},
		Metadata: output.LogRecordMetadata{
			Timestamp: data.timestamp,
			Severity:  data.severity,
		},
	}, nil
}

// parseApacheCLF parses an Apache CLF line into a map
func parseApacheCLF(line string) (map[string]any, error) {
	// Apache CLF format: remotehost rfc931 authuser [date] "request" status bytes
	// This is a simplified parser - real CLF can be more complex
	parts := strings.Fields(line)
	if len(parts) < 7 {
		return nil, fmt.Errorf("invalid CLF format: expected at least 7 fields, got %d", len(parts))
	}

	result := make(map[string]any)
	result["remote_host"] = parts[0]
	result["identity"] = parts[1]
	result["user_id"] = parts[2]

	// Parse timestamp [dd/MMM/yyyy:HH:mm:ss -TZ]
	if len(parts) > 3 && strings.HasPrefix(parts[3], "[") {
		timestampStr := strings.Trim(parts[3], "[]")
		result["timestamp"] = timestampStr
	}

	// Parse request "METHOD PATH PROTOCOL"
	if len(parts) > 4 && strings.HasPrefix(parts[4], `"`) {
		requestStr := strings.Trim(parts[4], `"`)
		requestParts := strings.Fields(requestStr)
		if len(requestParts) >= 3 {
			result["method"] = requestParts[0]
			result["path"] = requestParts[1]
			result["protocol"] = requestParts[2]
		}
		result["request"] = requestStr
	}

	// Parse status code
	if len(parts) > 5 {
		if status, err := strconv.Atoi(parts[5]); err == nil {
			result["status"] = status
		}
	}

	// Parse size
	if len(parts) > 6 {
		if size, err := strconv.Atoi(parts[6]); err == nil {
			result["size"] = size
		}
	}

	return result, nil
}

// recordWriteError records metrics for write errors
func (g *ApacheLogGenerator) recordWriteError(errorType string, _ error) {
	generator.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
	)
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *ApacheLogGenerator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
