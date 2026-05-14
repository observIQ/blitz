package nginx

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
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
	remoteAddr    string
	remoteUser    string
	timestamp     time.Time
	request       string
	statusCode    int
	bodyBytes     int
	referer       string
	userAgent     string
	severity      string
	requestTime   float64
	xForwardedFor string
}

// nginxLogRe matches the extended NGINX Combined Log Format with request_time and x_forwarded_for.
var nginxLogRe = regexp.MustCompile(`^(\S+) - (\S+) \[([^\]]+)\] "([^"]*)" (\d+) (\d+) "([^"]*)" "([^"]*)" ([\d.]+) "([^"]*)"$`)

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
		"/api/v1/users/profile/settings",
		"/api/v2/analytics/reports/summary",
		"/api/v1/orders/history/recent",
		"/api/v2/recommendations/personalized",
		"/api/v1/notifications/preferences",
		"/api/v2/search/advanced/filters",
		"/api/v1/subscriptions/billing/invoices",
		"/api/v2/integrations/webhooks/events",
		"/api/v1/admin/users/permissions/roles",
		"/api/v2/metrics/performance/aggregated",
		"/api/v1/catalog/products/featured",
		"/api/v2/reports/exports/scheduled",
		"/api/v1/account/security/mfa",
		"/api/v2/workflows/tasks/assignments",
		"/api/v1/content/media/uploads",
	}

	queryStrings = []string{
		"?page=1&limit=25&sort=created_at&order=desc",
		"?filter=active&category=electronics&min_price=10.00&max_price=500.00",
		"?q=search+term&lang=en&results=20&offset=0",
		"?user_id=12345&include=profile%2Csettings&format=json",
		"?status=pending&from=2024-01-01&to=2024-12-31&page=1&limit=100",
		"?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9&refresh=true",
		"?fields=id%2Cname%2Cemail%2Ccreated_at&page=2&limit=50",
		"?utm_source=google&utm_medium=cpc&utm_campaign=spring_sale&utm_content=banner",
		"?session_id=abc123def456789&redirect_uri=%2Fdashboard&state=xyz789",
		"?expand=orders%2Cpayments%2Caddress&include_deleted=false&version=2",
		"?q=product+search&category=clothing&size=M&color=blue&brand=nike&sort=price_asc",
		"?start_date=2024-01-01T00%3A00%3A00Z&end_date=2024-12-31T23%3A59%3A59Z&interval=daily",
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
		"/search?q=opentelemetry+collector&category=tools",
		"/products?category=electronics&sort=price_asc&page=2",
		"/blog/posts?tag=observability&limit=10&page=1",
		"/docs/api/v2/reference?section=authentication",
		"/dashboard?view=analytics&period=last30days&metric=requests",
		"/shop?department=networking&brand=cisco&in_stock=true&page=3",
		"/articles?topic=distributed-systems&author=staff&year=2024",
		"/account/orders?status=shipped&from=2024-01-01&limit=50",
		"/pricing?plan=enterprise&billing=annual&seats=50",
		"/docs/guides/getting-started?lang=go&version=v2",
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
	data.requestTime = float64(r.Intn(5000)+1) / 1000.0 // #nosec G404
	data.xForwardedFor = generateXForwardedFor(r)

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

// generateXForwardedFor generates a comma-separated chain of 1-4 proxy IPs
func generateXForwardedFor(r *rand.Rand) string {
	n := r.Intn(4) + 1 // #nosec G404
	ips := make([]string, n)
	for i := range ips {
		ips[i] = generateRandomIP(r)
	}
	return strings.Join(ips, ", ")
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

	// Attach a query string 85% of the time for non-root, non-static paths
	if r.Float64() < 0.85 && path != "/" && !strings.HasSuffix(path, ".html") { // #nosec G404
		path += queryStrings[r.Intn(len(queryStrings))] // #nosec G404
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

	nginxLine := fmt.Sprintf(`%s - %s [%s] "%s" %d %d "%s" "%s" %.3f "%s"`,
		data.remoteAddr,
		data.remoteUser,
		timestampStr,
		data.request,
		data.statusCode,
		data.bodyBytes,
		data.referer,
		data.userAgent,
		data.requestTime,
		data.xForwardedFor,
	)

	return output.LogRecord{
		Message: nginxLine,
		ParseFunc: func(message string) (map[string]any, error) {
			m := nginxLogRe.FindStringSubmatch(message)
			if m == nil {
				return nil, fmt.Errorf("invalid NGINX Combined log format: %q", message)
			}
			return map[string]any{
				"remote_addr":     m[1],
				"remote_user":     m[2],
				"time_local":      m[3],
				"request":         m[4],
				"status":          m[5],
				"body_bytes_sent": m[6],
				"http_referer":    m[7],
				"http_user_agent": m[8],
				"request_time":    m[9],
				"x_forwarded_for": m[10],
			}, nil
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
