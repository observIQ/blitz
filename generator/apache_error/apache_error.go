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
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const componentName = "apache-error"

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
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	consumer embed.LogConsumer
	static   *resource.StaticResources
	wg       sync.WaitGroup
	stopCh   chan struct{}
	tracker  *count.Tracker
	metrics  *generator.Metrics
}

// New creates a new Apache Error log generator. The consumer receives
// each generated record as a size-1 batch via ConsumeLogs.
func New(logger *zap.Logger, workers int, rate time.Duration, consumer embed.LogConsumer, tel embed.TelemetrySettings) (*ApacheErrorLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}

	metrics, err := generator.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build generator metrics: %w", err)
	}

	return &ApacheErrorLogGenerator{
		logger:   logger,
		workers:  workers,
		rate:     rate,
		consumer: consumer,
		static:   resource.FromIdentity(nil, "apache", "apache.format", "error"),
		metrics:  metrics,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier.
func (g *ApacheErrorLogGenerator) Name() string { return componentName }

// SetHostIdentity sets the simulated host whose identity every emitted record
// carries (PIPE-1036). A nil identity keeps the process-hostname fallback. Must
// be called before Start; the resource it builds is read concurrently by
// workers thereafter.
func (g *ApacheErrorLogGenerator) SetHostIdentity(id *datagen.SystemIdentity) {
	g.static = resource.FromIdentity(id, "apache", "apache.format", "error")
}

// Start launches the worker goroutines that push generated records to
// the configured consumer.
func (g *ApacheErrorLogGenerator) Start(_ context.Context) error {
	g.logger.Info("Starting Apache Error log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i) // #nosec G118 -- workers are bounded by Stop() and the WaitGroup, not the Start context
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

// SetCountTracker sets the finite generation count tracker.
func (g *ApacheErrorLogGenerator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

// worker is the main worker loop that generates and writes logs
func (g *ApacheErrorLogGenerator) worker(workerID int) {
	defer g.wg.Done()

	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 1, componentName)
	defer g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 0, componentName)

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0 // Never stop retrying

	// Drive the timer from this goroutine only. backoff.ExponentialBackOff is
	// not safe for concurrent use, so we never hand it to backoff.NewTicker's
	// internal goroutine; instead we own every NextBackOff/Reset call here.
	timer := time.NewTimer(backoffConfig.NextBackOff())
	defer timer.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker stopping", zap.Int("worker_id", workerID))
			return
		case <-timer.C:
			if g.tracker != nil && !g.tracker.Acquire() {
				select {
				case <-g.stopCh:
					return
				case <-g.tracker.ResumeC():
					timer.Reset(backoffConfig.NextBackOff())
					continue
				}
			}
			err := g.generateAndWriteLog(workerID)
			if err != nil {
				g.logger.Error("Failed to write log",
					zap.Int("worker_id", workerID),
					zap.Error(err))
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
		}
	}
}

// generateAndWriteLog generates a random log and pushes it as a
// single-record batch to the configured consumer.
func (g *ApacheErrorLogGenerator) generateAndWriteLog(_ int) error {
	// Generate Apache Error log data
	logData, err := g.generateApacheErrorLogData()
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("generate Apache Error log data: %w", err)
	}

	// Format log data as Apache Error Log Format
	logRecord, err := formatAsApacheError(logData, g.static)
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("format log as Apache Error: %w", err)
	}

	// Record logs generated counter
	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	// Push as a size-1 batch with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{logRecord}); err != nil {
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
		data.client = datagen.RandomIPv4(r)
	} else {
		data.client = ""
	}

	// Generate error message
	data.message = generateErrorMessage(r, data.level)

	return data, nil
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
func formatAsApacheError(data *apacheErrorLogData, static *resource.StaticResources) (embed.LogRecord, error) {
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

	return embed.LogRecord{
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
		Metadata: embed.LogRecordMetadata{
			Timestamp: data.timestamp,
			Severity:  data.severity,
			Resource:  static.Record(),
		},
	}, nil
}

// recordWriteError records a write error metric
func (g *ApacheErrorLogGenerator) recordWriteError(errorType string, err error) {
	g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
	)
	g.logger.Debug("Recorded write error",
		zap.String("error_type", errorType),
		zap.Error(err),
	)
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *ApacheErrorLogGenerator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
