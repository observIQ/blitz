package json

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	jsonlib "github.com/goccy/go-json"
	"github.com/observiq/blitz/internal/generator/logtypes"
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	// LogTypeDefault is the default log type
	LogTypeDefault = logtypes.LogTypeDefault
	// LogTypePII is the PII log type
	LogTypePII = logtypes.LogTypePII
)

// JSONLogGenerator generates JSON log data with configurable workers
type JSONLogGenerator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration
	logType string
	wg      sync.WaitGroup
	stopCh  chan struct{}
	meter   metric.Meter

	// Metrics
	jsonLogsGenerated metric.Int64Counter
	jsonActiveWorkers metric.Int64Gauge
	jsonWriteErrors   metric.Int64Counter
}

// New creates a new JSON log generator
func New(logger *zap.Logger, workers int, rate time.Duration, logType string) (*JSONLogGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	// Default to "default" if logType is empty
	if logType == "" {
		logType = LogTypeDefault
	}

	// Validate log type
	if logType != LogTypeDefault && logType != LogTypePII {
		return nil, fmt.Errorf("logType must be one of: %s, %s, got %q", LogTypeDefault, LogTypePII, logType)
	}

	meter := otel.Meter("blitz-generator")

	// Initialize metrics
	jsonLogsGenerated, err := meter.Int64Counter(
		"blitz.generator.logs.generated",
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	jsonActiveWorkers, err := meter.Int64Gauge(
		"blitz.generator.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	jsonWriteErrors, err := meter.Int64Counter(
		"blitz.generator.write.errors",
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	return &JSONLogGenerator{
		logger:            logger,
		workers:           workers,
		rate:              rate,
		logType:           logType,
		stopCh:            make(chan struct{}),
		meter:             meter,
		jsonLogsGenerated: jsonLogsGenerated,
		jsonActiveWorkers: jsonActiveWorkers,
		jsonWriteErrors:   jsonWriteErrors,
	}, nil
}

// Start starts the JSON log generator and writes data using the
// provided generator writer.
func (g *JSONLogGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting JSON log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate))

	// Record initial active workers count
	g.jsonActiveWorkers.Record(context.Background(), int64(g.workers),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_json"),
			),
		),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the JSON log generator.
// This function expects to be called exactly once.
func (g *JSONLogGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping JSON log generator")

	// Record zero active workers
	g.jsonActiveWorkers.Record(ctx, 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_json"),
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
func (g *JSONLogGenerator) worker(workerID int, writer output.Writer) {
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
func (g *JSONLogGenerator) generateAndWriteLog(writer output.Writer, workerID int) error {
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
	g.jsonLogsGenerated.Add(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_json"),
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

// formatAsJSON converts LogData to a JSON-formatted LogRecord
func formatAsJSON(data logtypes.LogData) (output.LogRecord, error) {
	var jsonData any
	var timestamp time.Time
	var severity string

	switch d := data.(type) {
	case *logtypes.DefaultLogData:
		jsonData = map[string]any{
			"timestamp":   d.TimestampVal,
			"level":       d.LevelVal,
			"environment": d.EnvironmentVal,
			"location":    d.LocationVal,
			"message":     d.MessageVal,
		}
		timestamp = d.TimestampVal
		severity = d.LevelVal
	case *logtypes.PIILogData:
		jsonData = formatPIILogData(d)
		timestamp = d.TimestampVal
		severity = d.LevelVal
	default:
		return output.LogRecord{}, fmt.Errorf("unsupported log data type: %T", data)
	}

	b, err := jsonlib.Marshal(jsonData)
	if err != nil {
		return output.LogRecord{}, fmt.Errorf("marshal JSON log: %w", err)
	}

	return output.LogRecord{
		Message: string(b),
		ParseFunc: func(message string) (map[string]any, error) {
			var parsed map[string]any
			if err := jsonlib.Unmarshal([]byte(message), &parsed); err != nil {
				return nil, fmt.Errorf("unmarshal JSON log: %w", err)
			}
			return parsed, nil
		},
		Metadata: output.LogRecordMetadata{
			Timestamp: timestamp,
			Severity:  severity,
		},
	}, nil
}

// recordWriteError records metrics for write errors
func (g *JSONLogGenerator) recordWriteError(errorType string, err error) {
	ctx := context.Background()

	g.jsonWriteErrors.Add(ctx, 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "generator_json"),
				attribute.String("error_type", errorType),
			),
		),
	)
}

// formatPIILogData formats PII log data with a random selection of 1-5 PII fields
func formatPIILogData(d *logtypes.PIILogData) map[string]any {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404

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
		"timestamp": d.TimestampVal,
		"level":     d.LevelVal,
		"message":   d.MessageVal,
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

	// Select 1-5 random PII fields
	numFields := r.Intn(5) + 1 // #nosec G404
	for i := 0; i < numFields && i < len(piiFields); i++ {
		jsonData[piiFields[i].key] = piiFields[i].value
	}

	return jsonData
}
