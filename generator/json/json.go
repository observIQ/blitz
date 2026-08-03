package json

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	jsonlib "github.com/goccy/go-json"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/generator/logtypes"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	componentName = "json"

	// LogTypeDefault is the default log type
	LogTypeDefault = logtypes.LogTypeDefault
	// LogTypePII is the PII log type
	LogTypePII = logtypes.LogTypePII
)

// Operational context pools used when formatting PII logs (mirrors logtypes defaults)
var (
	jsonServices = []string{
		"api-gateway", "auth-service", "payment-service", "user-service",
		"notification-service", "data-pipeline", "cache-service", "search-service",
		"event-bus", "analytics-service", "order-service", "inventory-service",
	}
	jsonHosts = []string{
		"web-prod-01.us-east1.example.com", "web-prod-02.us-east1.example.com",
		"web-prod-01.us-west1.example.com", "worker-prod-01.eu-west1.example.com",
		"api-prod-01.us-east1.example.com", "api-prod-02.us-west1.example.com",
		"db-worker-01.us-east1.example.com", "cache-prod-01.us-east1.example.com",
		"batch-worker-01.us-east1.example.com", "stream-proc-01.eu-central1.example.com",
	}
	jsonComponents = []string{
		"http-handler", "database", "cache", "message-queue",
		"grpc-server", "storage", "scheduler", "auth-middleware",
	}
	jsonVersions = []string{"1.0.0", "1.1.0", "1.2.3", "2.0.0", "2.1.1", "3.0.0-rc1"}
	jsonRegions  = []string{
		"us-east-1", "us-west-2", "eu-west-1", "eu-central-1",
		"ap-southeast-1", "ap-northeast-1", "ca-central-1", "sa-east-1",
	}
)

func generateUUID(r *rand.Rand) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		r.Uint32(),
		r.Intn(0x10000),           // #nosec G404
		r.Intn(0x10000),           // #nosec G404
		r.Intn(0x10000),           // #nosec G404
		r.Int63n(0x1000000000000), // #nosec G404
	)
}

func generateTraceID(r *rand.Rand) string {
	return fmt.Sprintf("%016x%016x", r.Int63(), r.Int63()) // #nosec G404
}

func generateSpanID(r *rand.Rand) string {
	return fmt.Sprintf("%016x", r.Int63()) // #nosec G404
}

// JSONLogGenerator generates JSON log data with configurable workers
type JSONLogGenerator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	logType  string
	consumer embed.LogConsumer
	wg       sync.WaitGroup
	stopCh   chan struct{}
	tracker  *count.Tracker
}

// New creates a new JSON log generator. The consumer receives each
// generated record as a size-1 batch via ConsumeLogs.
func New(logger *zap.Logger, workers int, rate time.Duration, logType string, consumer embed.LogConsumer) (*JSONLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}

	// Default to "default" if logType is empty
	if logType == "" {
		logType = LogTypeDefault
	}

	// Validate log type
	if logType != LogTypeDefault && logType != LogTypePII {
		return nil, fmt.Errorf("logType must be one of: %s, %s, got %q", LogTypeDefault, LogTypePII, logType)
	}

	return &JSONLogGenerator{
		logger:   logger,
		workers:  workers,
		rate:     rate,
		logType:  logType,
		consumer: consumer,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier.
func (g *JSONLogGenerator) Name() string { return componentName }

// Start launches the worker goroutines that push generated records to
// the configured consumer.
func (g *JSONLogGenerator) Start(_ context.Context) error {
	g.logger.Info("Starting JSON log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate))

	// Record initial active workers count
	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i)
	}

	return nil
}

// Stop stops the JSON log generator.
// This function expects to be called exactly once.
func (g *JSONLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping JSON log generator")

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
func (g *JSONLogGenerator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

// worker runs a single worker goroutine
func (g *JSONLogGenerator) worker(workerID int) {
	defer g.wg.Done()

	g.logger.Debug("Starting worker", zap.Int("worker_id", workerID))

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
func (g *JSONLogGenerator) generateAndWriteLog(_ int) error {
	var logData logtypes.LogData
	var err error

	switch g.logType {
	case LogTypePII:
		var piiData *logtypes.PIILogData
		piiData, err = logtypes.GeneratePIIData()
		if err == nil {
			logData = piiData
		}
	default:
		var defaultData *logtypes.DefaultLogData
		defaultData, err = logtypes.GenerateDefaultLogData()
		if err == nil {
			logData = defaultData
		}
	}

	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("generate random log: %w", err)
	}

	// Format log data as JSON
	logRecord, err := formatAsJSON(logData)
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("format log as JSON: %w", err)
	}

	// Record logs generated counter
	generator.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	// Write the data with timeout
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

// formatAsJSON converts LogData to a JSON-formatted LogRecord
func formatAsJSON(data logtypes.LogData) (embed.LogRecord, error) {
	var jsonData any
	var timestamp time.Time
	var severity string
	var jsonType string

	switch d := data.(type) {
	case *logtypes.DefaultLogData:
		jsonData = map[string]any{
			"timestamp":      d.TimestampVal,
			"level":          d.LevelVal,
			"environment":    d.EnvironmentVal,
			"location":       d.LocationVal,
			"message":        d.MessageVal,
			"service":        d.ServiceVal,
			"host":           d.HostVal,
			"request_id":     d.RequestIDVal,
			"trace_id":       d.TraceIDVal,
			"span_id":        d.SpanIDVal,
			"duration_ms":    d.DurationMsVal,
			"component":      d.ComponentVal,
			"version":        d.VersionVal,
			"correlation_id": d.CorrelationIDVal,
			"region":         d.RegionVal,
		}
		timestamp = d.TimestampVal
		severity = d.LevelVal
		jsonType = LogTypeDefault
	case *logtypes.PIILogData:
		jsonData = formatPIILogData(d)
		timestamp = d.TimestampVal
		severity = d.LevelVal
		jsonType = LogTypePII
	default:
		return embed.LogRecord{}, fmt.Errorf("unsupported log data type: %T", data)
	}

	b, err := jsonlib.Marshal(jsonData)
	if err != nil {
		return embed.LogRecord{}, fmt.Errorf("marshal JSON log: %w", err)
	}

	return embed.LogRecord{
		Message: string(b),
		ParseFunc: func(message string) (map[string]any, error) {
			var parsed map[string]any
			if err := jsonlib.Unmarshal([]byte(message), &parsed); err != nil {
				return nil, fmt.Errorf("unmarshal JSON log: %w", err)
			}
			return parsed, nil
		},
		Metadata: embed.LogRecordMetadata{
			Timestamp: timestamp,
			Severity:  severity,
			Resource:  resource.Default(componentName, "json.type", jsonType),
		},
	}, nil
}

// recordWriteError records metrics for write errors
func (g *JSONLogGenerator) recordWriteError(errorType string, _ error) {
	generator.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
	)
}

// formatPIILogData formats PII log data with a random selection of 1-5 PII fields
func formatPIILogData(d *logtypes.PIILogData) map[string]any {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404
	return formatPIILogDataWithRand(r, d)
}

// formatPIILogDataWithRand formats PII log data using the provided rand source
func formatPIILogDataWithRand(r *rand.Rand, d *logtypes.PIILogData) map[string]any {
	// All available PII fields
	piiFields := []struct {
		key   string
		value any
	}{
		// Core PII
		{"user_id", d.UserIDVal},
		{"ssn", d.SSNVal},
		{"iban", d.IBANVal},
		{"phone", d.PhoneVal},
		{"intl_phone", d.IntlPhoneVal},
		{"email", d.EmailVal},
		{"credit_card", d.CreditCardVal},
		{"dob", d.DOBVal},
		{"ipv4", d.IPv4Val},
		{"ipv6", d.IPv6Val},
		{"mac_address", d.MACAddressVal},
		{"street_addr", d.StreetAddrVal},
		{"city_state", d.CityStateVal},
		{"zip_code", d.ZipCodeVal},

		// Government IDs
		{"passport", d.PassportVal},
		{"drivers_license", d.DriversLicenseVal},
		{"national_id", d.NationalIDVal},

		// Financial
		{"bank_account", d.BankAccountVal},
		{"routing_number", d.RoutingNumberVal},
		{"crypto_wallet", d.CryptoWalletVal},

		// Healthcare
		{"medical_record", d.MedicalRecordVal},
		{"health_insurance", d.HealthInsuranceVal},

		// Vehicle
		{"vin", d.VINVal},
		{"license_plate", d.LicensePlateVal},

		// Employment/Education
		{"employee_id", d.EmployeeIDVal},
		{"student_id", d.StudentIDVal},

		// Authentication/Secrets
		{"username", d.UsernameVal},
		{"password_hash", d.PasswordHashVal},
		{"api_key", d.APIKeyVal},
		{"aws_access_key", d.AWSAccessKeyVal},
		{"private_key", d.PrivateKeyVal},
		{"jwt_token", d.JWTTokenVal},

		// Location
		{"gps_coords", d.GPSCoordsVal},
		{"geohash", d.GeohashVal},

		// Personal
		{"full_name", d.FullNameVal},
		{"mothers_maiden", d.MothersMaidenVal},
		{"security_answer", d.SecurityAnswerVal},
	}

	// Start with base fields
	jsonData := map[string]any{
		"timestamp":      d.TimestampVal,
		"level":          d.LevelVal,
		"message":        d.MessageVal,
		"service":        jsonServices[r.Intn(len(jsonServices))], // #nosec G404
		"host":           jsonHosts[r.Intn(len(jsonHosts))],       // #nosec G404
		"request_id":     generateUUID(r),
		"trace_id":       generateTraceID(r),
		"span_id":        generateSpanID(r),
		"duration_ms":    r.Intn(9999) + 1,                            // #nosec G404
		"component":      jsonComponents[r.Intn(len(jsonComponents))], // #nosec G404
		"version":        jsonVersions[r.Intn(len(jsonVersions))],     // #nosec G404
		"correlation_id": generateUUID(r),
		"region":         jsonRegions[r.Intn(len(jsonRegions))], // #nosec G404
	}

	// Add optional context fields if present
	if d.EventVal != "" {
		jsonData["event"] = d.EventVal
	}
	if d.DetailVal != "" {
		jsonData["detail"] = d.DetailVal
	}
	if d.TypeVal != "" {
		jsonData["type"] = d.TypeVal
	}
	if d.ActionVal != "" {
		jsonData["action"] = d.ActionVal
	}
	if d.StatusVal != "" {
		jsonData["status"] = d.StatusVal
	}

	// Shuffle the PII fields
	r.Shuffle(len(piiFields), func(i, j int) {
		piiFields[i], piiFields[j] = piiFields[j], piiFields[i]
	})

	// Select 3-8 random PII fields
	numFields := r.Intn(6) + 3 // #nosec G404
	for i := 0; i < numFields && i < len(piiFields); i++ {
		jsonData[piiFields[i].key] = piiFields[i].value
	}

	return jsonData
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *JSONLogGenerator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
