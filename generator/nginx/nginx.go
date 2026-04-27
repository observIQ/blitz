package nginx

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/generator/security"
	"github.com/observiq/blitz/internal/useragent"
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	componentName = "nginx"

	// error types
	errorTypeUnknown = "unknown"
	errorTypeTimeout = "timeout"

	// severity levels
	severityInfo  = "INFO"
	severityWarn  = "WARN"
	severityError = "ERROR"

	// empty value indicator
	emptyValue = "-"
)

// nginxLogData represents the data needed to generate an NGINX Combined Log Format entry
type nginxLogData struct {
	remoteAddr string
	remoteUser string
	timestamp  time.Time
	request    string
	statusCode int
	bodyBytes  int
	referer    string
	userAgent  string
	severity   string
}

// Generator generates NGINX Combined Log Format log data
type Generator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
}

// Predefined lists for fast random generation
var (
	httpMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	httpPaths = []string{
		"/",
		"/index.html",
		"/api/v1/users",
		"/api/v1/orders",
		"/api/v1/products",
		"/api/v1/inventory",
		"/api/v1/customers",
		"/api/v1/payments",
		"/api/v1/transactions",
		"/api/v1/accounts",
		"/api/v1/auth",
		"/api/v1/loans",
		"/api/v1/transfers",
		"/api/v1/verification",
		"/api/v2/data",
		"/health",
		"/status",
		"/about",
		"/contact",
		"/search",
		"/login",
		"/logout",
		"/dashboard",
		"/profile",
		"/settings",
	}

	httpProtocols = []string{"HTTP/1.0", "HTTP/1.1", "HTTP/2.0"}

	statusCodes2xx = []int{200, 201, 204}
	statusCodes4xx = []int{400, 401, 403, 404, 429}
	statusCodes5xx = []int{500, 502, 503, 504}

	refererDomains = []string{
		"https://www.example.com",
		"https://search.example.com",
		"https://www.google.com",
		"https://www.bing.com",
		"https://github.com",
		"https://stackoverflow.com",
		"https://www.reddit.com",
		"https://www.linkedin.com",
	}

	refererPages = []string{
		"/",
		"/search",
		"/page1",
		"/page2",
		"/index.html",
		"/products",
		"/about",
		"/contact",
	}

	remoteUsers = []string{"-", "admin", "user1", "user2", "guest"}
)

// New creates a new NGINX log generator
func New(logger *zap.Logger, workers int, rate time.Duration) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	return &Generator{
		logger:  logger,
		workers: workers,
		rate:    rate,
		stopCh:  make(chan struct{}),
	}, nil
}

// Start starts the NGINX log generator and writes data using the
// provided generator writer.
func (g *Generator) Start(writer output.Writer) error {
	g.logger.Info("Starting NGINX log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the NGINX log generator and waits for all workers to finish.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping NGINX log generator")

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("NGINX log generator stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SetCountTracker sets the finite generation count tracker.
func (g *Generator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

// worker is the main worker loop that generates and writes logs
func (g *Generator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()

	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 1, componentName)
	defer generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 0, componentName)

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0

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
func (g *Generator) generateAndWriteLog(writer output.Writer, workerID int) error {
	logData, err := g.generateNginxLogData()
	if err != nil {
		g.recordWriteError(errorTypeUnknown, err)
		return fmt.Errorf("generate NGINX log data: %w", err)
	}

	logRecord, err := formatAsNginxCombined(logData)
	if err != nil {
		g.recordWriteError(errorTypeUnknown, err)
		return fmt.Errorf("format log as NGINX Combined: %w", err)
	}

	generator.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writer.Write(ctx, logRecord); err != nil {
		errorType := errorTypeUnknown
		if ctx.Err() == context.DeadlineExceeded {
			errorType = errorTypeTimeout
		}
		g.recordWriteError(errorType, err)
		return err
	}

	return nil
}

// generateNginxLogData generates random NGINX Combined log data
func (g *Generator) generateNginxLogData() (*nginxLogData, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	data := &nginxLogData{
		timestamp: time.Now(),
	}

	data.remoteAddr = generateRandomIP(r)
	data.remoteUser = remoteUsers[r.Intn(len(remoteUsers))] // #nosec G404
	data.request = generateRequest(r)
	data.statusCode, data.severity = generateStatusAndSeverity(r)
	data.bodyBytes = r.Intn(10000000) + 100 // #nosec G404
	data.referer = generateReferer(r)
	data.userAgent = useragent.RandomUserAgent(r)

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
	method := httpMethods[r.Intn(len(httpMethods))] // #nosec G404

	// 20% chance of generating a security-focused path
	var path string
	if r.Float64() < 0.20 { // #nosec G404
		path = security.RandomAttackPath(r)
	} else {
		path = httpPaths[r.Intn(len(httpPaths))] // #nosec G404
	}

	protocol := httpProtocols[r.Intn(len(httpProtocols))] // #nosec G404

	return fmt.Sprintf("%s %s %s", method, path, protocol)
}

// generateStatusAndSeverity generates a random HTTP status code and corresponding severity
func generateStatusAndSeverity(r *rand.Rand) (int, string) {
	roll := r.Float64() // #nosec G404

	switch {
	case roll < 0.85:
		status := statusCodes2xx[r.Intn(len(statusCodes2xx))] // #nosec G404
		return status, severityInfo
	case roll < 0.95:
		status := statusCodes4xx[r.Intn(len(statusCodes4xx))] // #nosec G404
		return status, severityWarn
	default:
		status := statusCodes5xx[r.Intn(len(statusCodes5xx))] // #nosec G404
		return status, severityError
	}
}

// generateReferer generates a random referer URL
func generateReferer(r *rand.Rand) string {
	if r.Float64() < 0.3 { // #nosec G404
		return emptyValue
	}

	domain := refererDomains[r.Intn(len(refererDomains))] // #nosec G404
	page := refererPages[r.Intn(len(refererPages))]       // #nosec G404

	return fmt.Sprintf("%s%s", domain, page)
}

// formatAsNginxCombined converts nginxLogData to NGINX Combined Log Format
// Format: $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
// Example: 127.0.0.1 - - [25/Dec/2023:10:15:30 -0800] "GET /index.html HTTP/1.1" 200 2326 "https://www.example.com/" "Mozilla/5.0..."
func formatAsNginxCombined(data *nginxLogData) (output.LogRecord, error) {
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

	nginxLine := fmt.Sprintf(`%s - %s [%s] "%s" %d %d "%s" "%s"`,
		data.remoteAddr,
		data.remoteUser,
		timestampStr,
		data.request,
		data.statusCode,
		data.bodyBytes,
		data.referer,
		data.userAgent,
	)

	return output.LogRecord{
		Message: nginxLine,
		ParseFunc: func(message string) (map[string]any, error) {
			parts := strings.Fields(message)
			if len(parts) < 9 {
				return nil, fmt.Errorf("invalid NGINX Combined log format: expected at least 9 fields, got %d", len(parts))
			}

			parsed := make(map[string]any)
			parsed["remote_addr"] = parts[0]
			parsed["remote_user"] = parts[2]
			if len(parts) > 3 {
				parsed["time_local"] = strings.Trim(parts[3], "[]")
			}
			if len(parts) > 4 {
				parsed["request"] = strings.Trim(parts[4], `"`)
			}
			if len(parts) > 5 {
				parsed["status"] = parts[5]
			}
			if len(parts) > 6 {
				parsed["body_bytes_sent"] = parts[6]
			}
			if len(parts) > 7 {
				parsed["http_referer"] = strings.Trim(parts[7], `"`)
			}
			if len(parts) > 8 {
				parsed["http_user_agent"] = strings.Trim(parts[8], `"`)
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
func (g *Generator) recordWriteError(errorType string, err error) {
	generator.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
	)
	g.logger.Debug("Recorded write error",
		zap.String("error_type", errorType),
		zap.Error(err),
	)
}
