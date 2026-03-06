package apache_error

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
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// apacheErrorLogData represents the data needed to generate an Apache error log entry
type apacheErrorLogData struct {
	timestamp time.Time
	level     string
	pid       int
	tid       int
	client    string
	message   string
	severity  string
}

// ApacheErrorLogGenerator generates Apache Error Log Format log data
type ApacheErrorLogGenerator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	meter   metric.Meter

	// Metrics
	apacheErrorLogsGenerated metric.Int64Counter
	apacheErrorActiveWorkers metric.Int64Gauge
	apacheErrorWriteErrors   metric.Int64Counter
}

// New creates a new Apache Error log generator
func New(logger *zap.Logger, workers int, rate time.Duration) (*ApacheErrorLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	meter := otel.Meter("blitz-generator")

	// Initialize metrics
	apacheErrorLogsGenerated, err := meter.Int64Counter(
		"blitz.generator.logs.generated",
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	apacheErrorActiveWorkers, err := meter.Int64Gauge(
		"blitz.generator.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	apacheErrorWriteErrors, err := meter.Int64Counter(
		"blitz.generator.write.errors",
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	return &ApacheErrorLogGenerator{
		logger:                   logger,
		workers:                  workers,
		rate:                     rate,
		stopCh:                   make(chan struct{}),
		meter:                    meter,
		apacheErrorLogsGenerated: apacheErrorLogsGenerated,
		apacheErrorActiveWorkers: apacheErrorActiveWorkers,
		apacheErrorWriteErrors:   apacheErrorWriteErrors,
	}, nil
}

// Start starts the Apache Error log generator and writes data using the
// provided generator writer.
// SupportedTelemetry returns the telemetry types this generator supports.
func (g *ApacheErrorLogGenerator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}

func (g *ApacheErrorLogGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting Apache Error log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the Apache Error log generator and waits for all workers to finish.
func (g *ApacheErrorLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Apache Error log generator")

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("Apache Error log generator stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// worker is the main worker loop that generates and writes logs
func (g *ApacheErrorLogGenerator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()

	g.apacheErrorActiveWorkers.Record(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache_error"),
				attribute.Int("worker_id", workerID),
			),
		),
	)
	defer g.apacheErrorActiveWorkers.Record(context.Background(), 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache_error"),
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
func (g *ApacheErrorLogGenerator) generateAndWriteLog(writer output.Writer, workerID int) error {
	// Generate Apache Error log data
	logData, err := g.generateApacheErrorLogData()
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("generate Apache Error log data: %w", err)
	}

	// Format log data as Apache Error Log Format
	logRecord, err := formatAsApacheError(logData)
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("format log as Apache Error: %w", err)
	}

	// Record logs generated counter
	g.apacheErrorLogsGenerated.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache_error"),
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

// generateApacheErrorLogData generates random Apache Error log data
func (g *ApacheErrorLogGenerator) generateApacheErrorLogData() (*apacheErrorLogData, error) {
	// Use fast random generator with gosec nosec comment
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	data := &apacheErrorLogData{
		timestamp: time.Now(),
	}

	// Generate log level (error, warn, info, debug, notice, crit, alert, emerg)
	levels := []struct {
		level    string
		severity string
		weight   float64
	}{
		{"error", "ERROR", 0.40},
		{"warn", "WARN", 0.20},
		{"info", "INFO", 0.15},
		{"notice", "INFO", 0.10},
		{"debug", "DEBUG", 0.08},
		{"crit", "ERROR", 0.04},
		{"alert", "ERROR", 0.02},
		{"emerg", "ERROR", 0.01},
	}

	roll := r.Float64() // #nosec G404
	cumulative := 0.0
	for _, l := range levels {
		cumulative += l.weight
		if roll < cumulative {
			data.level = l.level
			data.severity = l.severity
			break
		}
	}

	// Generate process ID and thread ID
	data.pid = r.Intn(65535) + 1 // #nosec G404
	data.tid = r.Intn(10000)     // #nosec G404

	// Generate client IP (sometimes empty for non-client errors)
	if r.Float64() < 0.8 { // #nosec G404
		data.client = generateRandomIP(r)
	} else {
		data.client = ""
	}

	// Generate error message
	data.message = generateErrorMessage(r, data.level)

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

// generateErrorMessage generates a random error message based on log level
func generateErrorMessage(r *rand.Rand, level string) string {
	switch level {
	case "error":
		messages := []string{
			"File does not exist: %s",
			"Permission denied: %s",
			"AH00111: Config variable %s is not defined",
			"AH00558: httpd: Could not reliably determine the server's fully qualified domain name",
			"AH00098: pid file %s overwritten -- Unclean shutdown",
			"client denied by server configuration: %s",
			"Invalid method in request %s",
			"Request exceeded the limit of 10 internal redirects",
		}
		msg := messages[r.Intn(len(messages))] // #nosec G404
		paths := []string{
			"/var/www/html/index.php",
			"/export/home/live/ap/htdocs/test",
			"/usr/local/apache2/htdocs/config.php",
			"/home/user/public_html/.htaccess",
			"/var/www/example.com/public/index.html",
		}
		return fmt.Sprintf(msg, paths[r.Intn(len(paths))]) // #nosec G404

	case "warn":
		messages := []string{
			"Hostname %s provided via SNI and hostname %s provided via HTTP are different",
			"mod_rewrite: maximum number of internal redirects reached",
			"Long running script detected: %s",
			"Request header exceeds server limit",
			"Invalid URI in request %s",
		}
		msg := messages[r.Intn(len(messages))] // #nosec G404
		values := []string{
			"example.com",
			"www.example.com",
			"/api/v1/users",
			"/index.php",
			"GET /test HTTP/1.1",
		}
		return fmt.Sprintf(msg, values[r.Intn(len(values))]) // #nosec G404

	case "crit":
		messages := []string{
			"Server ran out of threads to serve requests",
			"Fatal error: Out of memory",
			"AH00052: child pid %d exit signal Segmentation fault (11)",
			"AH00020: ConfigDirectory %s/.htaccess cannot be accessed",
		}
		msg := messages[r.Intn(len(messages))] // #nosec G404
		if strings.Contains(msg, "%d") {
			return fmt.Sprintf(msg, r.Intn(10000)+1000) // #nosec G404
		}
		paths := []string{"/var/www", "/etc/apache2", "/usr/local/apache2"}
		return fmt.Sprintf(msg, paths[r.Intn(len(paths))]) // #nosec G404

	case "alert", "emerg":
		messages := []string{
			"AH00016: Configuration Failed",
			"AH00017: Pre-configuration failed, exiting",
			"AH00018: Unable to open logs",
			"AH00019: Couldn't create pchild mutex",
		}
		return messages[r.Intn(len(messages))] // #nosec G404

	case "notice":
		messages := []string{
			"Graceful restart requested, doing restart",
			"Resuming normal operations",
			"Server configured -- resuming normal operations",
			"caught SIGTERM, shutting down",
		}
		return messages[r.Intn(len(messages))] // #nosec G404

	case "info":
		messages := []string{
			"Server built: %s",
			"AH00094: Command line: '%s'",
			"AH00012: httpd: Could not open error log file %s",
		}
		msg := messages[r.Intn(len(messages))] // #nosec G404
		values := []string{
			"2024-01-15 10:30:00",
			"/usr/sbin/httpd -D FOREGROUND",
			"/var/log/apache2/error.log",
		}
		return fmt.Sprintf(msg, values[r.Intn(len(values))]) // #nosec G404

	default: // debug
		messages := []string{
			"proxy: HTTP: connection established with %s",
			"proxy: HTTP: connection closed to %s",
			"proxy: HTTP: connection established with %s",
		}
		msg := messages[r.Intn(len(messages))] // #nosec G404
		hosts := []string{
			"192.168.1.100:8080",
			"10.0.0.1:443",
			"example.com:80",
		}
		return fmt.Sprintf(msg, hosts[r.Intn(len(hosts))]) // #nosec G404
	}
}

// formatAsApacheError converts apacheErrorLogData to Apache Error Log Format
// Format: [timestamp] [level] [pid:tid] [client] message
// Example: [Wed Oct 11 14:32:52 2000] [error] [client 127.0.0.1] client denied by server configuration: /export/home/live/ap/htdocs/test
func formatAsApacheError(data *apacheErrorLogData) (output.LogRecord, error) {
	// Format timestamp as [Day Mon DD HH:MM:SS YYYY]
	timestampStr := data.timestamp.Format("[Mon Jan 02 15:04:05 2006]")

	// Build error log line
	var errorLine string
	if data.client != "" {
		errorLine = fmt.Sprintf(`%s [%s] [pid %d:tid %d] [client %s] %s`,
			timestampStr,
			data.level,
			data.pid,
			data.tid,
			data.client,
			data.message,
		)
	} else {
		// Some errors don't have a client (e.g., server startup/shutdown)
		errorLine = fmt.Sprintf(`%s [%s] [pid %d:tid %d] %s`,
			timestampStr,
			data.level,
			data.pid,
			data.tid,
			data.message,
		)
	}

	return output.LogRecord{
		Message: errorLine,
		ParseFunc: func(message string) (map[string]any, error) {
			// Basic parsing - extract bracketed fields
			parsed := make(map[string]any)

			// Extract timestamp (first bracketed field)
			parts := strings.Split(message, "]")
			if len(parts) > 0 {
				parsed["timestamp"] = strings.Trim(parts[0], "[")
			}

			// Extract level (second bracketed field)
			if len(parts) > 1 {
				parsed["level"] = strings.TrimSpace(parts[1])
			}

			// Extract pid:tid (third bracketed field)
			if len(parts) > 2 {
				pidTid := strings.TrimSpace(parts[2])
				parsed["pid_tid"] = pidTid
				// Try to extract pid and tid separately
				if strings.Contains(pidTid, "pid") {
					pidTidParts := strings.Fields(pidTid)
					for i, part := range pidTidParts {
						if part == "pid" && i+1 < len(pidTidParts) {
							pidStr := strings.TrimSuffix(pidTidParts[i+1], ":tid")
							if pid, err := strconv.Atoi(pidStr); err == nil {
								parsed["pid"] = pid
							}
						}
						if strings.HasPrefix(part, "tid") && i+1 < len(pidTidParts) {
							if tid, err := strconv.Atoi(pidTidParts[i+1]); err == nil {
								parsed["tid"] = tid
							}
						}
					}
				}
			}

			// Extract client (fourth bracketed field, if present)
			if len(parts) > 3 {
				clientPart := strings.TrimSpace(parts[3])
				if strings.HasPrefix(clientPart, "[client") {
					clientIP := strings.TrimPrefix(clientPart, "[client ")
					clientIP = strings.TrimSuffix(clientIP, "]")
					parsed["client"] = clientIP
				}
			}

			// Extract message (everything after the last bracket)
			if len(parts) > 1 {
				lastBracket := strings.LastIndex(message, "]")
				if lastBracket >= 0 && lastBracket+1 < len(message) {
					parsed["message"] = strings.TrimSpace(message[lastBracket+1:])
				}
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
func (g *ApacheErrorLogGenerator) recordWriteError(errorType string, err error) {
	g.apacheErrorWriteErrors.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_apache_error"),
				attribute.String("error_type", errorType),
			),
		),
	)
	g.logger.Debug("Recorded write error",
		zap.String("error_type", errorType),
		zap.Error(err),
	)
}
