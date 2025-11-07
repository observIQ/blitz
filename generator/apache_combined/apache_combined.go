package apache_combined

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

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
	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	meter   metric.Meter

	// Metrics
	apacheCombinedLogsGenerated metric.Int64Counter
	apacheCombinedActiveWorkers metric.Int64Gauge
	apacheCombinedWriteErrors   metric.Int64Counter
}

// New creates a new Apache Combined log generator
func New(logger *zap.Logger, workers int, rate time.Duration) (*ApacheCombinedLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	meter := otel.Meter("blitz-generator")

	// Initialize metrics
	apacheCombinedLogsGenerated, err := meter.Int64Counter(
		"blitz.generator.logs.generated",
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	apacheCombinedActiveWorkers, err := meter.Int64Gauge(
		"blitz.generator.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	apacheCombinedWriteErrors, err := meter.Int64Counter(
		"blitz.generator.write.errors",
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	return &ApacheCombinedLogGenerator{
		logger:                      logger,
		workers:                     workers,
		rate:                        rate,
		stopCh:                      make(chan struct{}),
		meter:                       meter,
		apacheCombinedLogsGenerated: apacheCombinedLogsGenerated,
		apacheCombinedActiveWorkers: apacheCombinedActiveWorkers,
		apacheCombinedWriteErrors:   apacheCombinedWriteErrors,
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

// worker is the main worker loop that generates and writes logs
func (g *ApacheCombinedLogGenerator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()

	g.apacheCombinedActiveWorkers.Record(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache_combined"),
				attribute.Int("worker_id", workerID),
			),
		),
	)
	defer g.apacheCombinedActiveWorkers.Record(context.Background(), 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache_combined"),
				attribute.Int("worker_id", workerID),
			),
		),
	)

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
	g.apacheCombinedLogsGenerated.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache_combined"),
			),
		),
	)

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

	// Generate referer
	data.referer = generateReferer(r)

	// Generate user agent
	data.userAgent = generateUserAgent(r)

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

	paths := []string{
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

	path := paths[r.Intn(len(paths))] // #nosec G404

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

// generateReferer generates a random referer URL
func generateReferer(r *rand.Rand) string {
	// Sometimes no referer (direct access)
	if r.Float64() < 0.3 { // #nosec G404
		return "-"
	}

	domains := []string{
		"https://www.example.com",
		"https://search.example.com",
		"https://www.google.com",
		"https://www.bing.com",
		"https://github.com",
		"https://stackoverflow.com",
	}

	pages := []string{
		"/",
		"/search",
		"/page1",
		"/page2",
		"/index.html",
		"/products",
		"/about",
	}

	domain := domains[r.Intn(len(domains))] // #nosec G404
	page := pages[r.Intn(len(pages))]       // #nosec G404

	return fmt.Sprintf("%s%s", domain, page)
}

// generateUserAgent generates a random user agent string
func generateUserAgent(r *rand.Rand) string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		"curl/7.68.0",
		"PostmanRuntime/7.32.3",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	}

	return userAgents[r.Intn(len(userAgents))] // #nosec G404
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
	g.apacheCombinedWriteErrors.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache_combined"),
				attribute.String("error_type", errorType),
			),
		),
	)
	g.logger.Debug("Recorded write error",
		zap.String("error_type", errorType),
		zap.Error(err),
	)
}
