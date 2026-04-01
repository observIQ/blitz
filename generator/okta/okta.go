package okta

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	jsonlib "github.com/goccy/go-json"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	componentName = "generator_okta"
	meterName     = "blitz-generator"

	metricLogsGenerated = "blitz.generator.logs.generated"
	metricWorkersActive = "blitz.generator.workers.active"
	metricWriteErrors   = "blitz.generator.write.errors"

	errorTypeUnknown = "unknown"
	errorTypeTimeout = "timeout"
)

// Generator generates Okta System Log format log data
type Generator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
	meter   metric.Meter

	oktaLogsGenerated metric.Int64Counter
	oktaActiveWorkers metric.Int64Gauge
	oktaWriteErrors   metric.Int64Counter
}

// Predefined data for realistic Okta log generation
var (
	eventTypes = []struct {
		eventType  string
		displayMsg string
		severity   string
		outcome    string
		category   string
	}{
		// Authentication events
		{"user.session.start", "User login to Okta", "INFO", "SUCCESS", "authentication"},
		{"user.session.end", "User logout from Okta", "INFO", "SUCCESS", "authentication"},
		{"user.authentication.sso", "User single sign on to app", "INFO", "SUCCESS", "authentication"},
		{"user.authentication.auth_via_mfa", "Authentication via MFA", "INFO", "SUCCESS", "authentication"},
		{"user.authentication.verify", "User attempted authentication", "INFO", "SUCCESS", "authentication"},

		// Authentication failures
		{"user.session.start", "User login to Okta", "WARN", "FAILURE", "authentication"},
		{"user.authentication.auth_via_mfa", "MFA verification failed", "WARN", "FAILURE", "authentication"},
		{"user.authentication.verify", "Authentication failed - invalid credentials", "WARN", "FAILURE", "authentication"},
		{"user.account.lock", "Account locked due to multiple failed attempts", "WARN", "SUCCESS", "authentication"},

		// Security events
		{"security.threat.detected", "Suspicious activity detected", "WARN", "SUCCESS", "security"},
		{"security.request.blocked", "Request blocked by security policy", "WARN", "SUCCESS", "security"},
		{"user.session.impersonation.start", "Admin impersonation session started", "WARN", "SUCCESS", "security"},
		{"user.session.impersonation.end", "Admin impersonation session ended", "INFO", "SUCCESS", "security"},

		// User lifecycle events
		{"user.lifecycle.create", "User created in Okta", "INFO", "SUCCESS", "user_management"},
		{"user.lifecycle.activate", "User activated", "INFO", "SUCCESS", "user_management"},
		{"user.lifecycle.deactivate", "User deactivated", "INFO", "SUCCESS", "user_management"},
		{"user.lifecycle.suspend", "User suspended", "WARN", "SUCCESS", "user_management"},
		{"user.lifecycle.unsuspend", "User unsuspended", "INFO", "SUCCESS", "user_management"},
		{"user.lifecycle.delete", "User deleted from Okta", "INFO", "SUCCESS", "user_management"},

		// Password events
		{"user.account.update_password", "User changed password", "INFO", "SUCCESS", "password"},
		{"user.account.reset_password", "Password reset requested", "INFO", "SUCCESS", "password"},
		{"user.credential.forgot_password", "Forgot password flow initiated", "INFO", "SUCCESS", "password"},

		// Application events
		{"app.user_membership.add", "User added to application", "INFO", "SUCCESS", "application"},
		{"app.user_membership.remove", "User removed from application", "INFO", "SUCCESS", "application"},
		{"application.lifecycle.create", "Application created", "INFO", "SUCCESS", "application"},
		{"application.lifecycle.update", "Application updated", "INFO", "SUCCESS", "application"},
		{"application.lifecycle.delete", "Application deleted", "INFO", "SUCCESS", "application"},

		// Group events
		{"group.user_membership.add", "User added to group", "INFO", "SUCCESS", "group"},
		{"group.user_membership.remove", "User removed from group", "INFO", "SUCCESS", "group"},
		{"group.lifecycle.create", "Group created", "INFO", "SUCCESS", "group"},
		{"group.lifecycle.delete", "Group deleted", "INFO", "SUCCESS", "group"},

		// Policy events
		{"policy.lifecycle.create", "Policy created", "INFO", "SUCCESS", "policy"},
		{"policy.lifecycle.update", "Policy updated", "INFO", "SUCCESS", "policy"},
		{"policy.lifecycle.delete", "Policy deleted", "INFO", "SUCCESS", "policy"},
		{"policy.rule.create", "Policy rule created", "INFO", "SUCCESS", "policy"},
		{"policy.rule.update", "Policy rule updated", "INFO", "SUCCESS", "policy"},

		// Admin events
		{"user.account.privilege.grant", "Admin privilege granted", "WARN", "SUCCESS", "admin"},
		{"user.account.privilege.revoke", "Admin privilege revoked", "INFO", "SUCCESS", "admin"},
		{"system.api_token.create", "API token created", "WARN", "SUCCESS", "admin"},
		{"system.api_token.revoke", "API token revoked", "INFO", "SUCCESS", "admin"},

		// Suspicious/Attack patterns
		{"user.session.start", "Login from suspicious location", "WARN", "FAILURE", "security"},
		{"user.session.start", "Login from new device", "INFO", "SUCCESS", "security"},
		{"security.threat.detected", "Brute force attack detected", "ERROR", "SUCCESS", "security"},
		{"security.threat.detected", "Credential stuffing attack detected", "ERROR", "SUCCESS", "security"},
		{"security.threat.detected", "Impossible travel detected", "WARN", "SUCCESS", "security"},
		{"user.mfa.factor.deactivate", "MFA factor removed", "WARN", "SUCCESS", "security"},
		{"user.account.unlock", "Account unlocked by admin", "INFO", "SUCCESS", "security"},
	}

	actors = []struct {
		displayName string
		login       string
		userType    string
	}{
		{"John Smith", "john.smith@example.com", "User"},
		{"Jane Doe", "jane.doe@example.com", "User"},
		{"Bob Wilson", "bob.wilson@example.com", "User"},
		{"Alice Johnson", "alice.johnson@example.com", "User"},
		{"System Admin", "admin@example.com", "Admin"},
		{"Security Admin", "security@example.com", "Admin"},
		{"Help Desk", "helpdesk@example.com", "Admin"},
		{"Service Account", "svc-account@example.com", "ServiceAccount"},
		{"API Client", "api-client@example.com", "ServiceAccount"},
		{"Unknown Actor", "unknown@suspicious.com", "User"},
	}

	applications = []struct {
		name  string
		label string
	}{
		{"salesforce", "Salesforce"},
		{"office365", "Microsoft Office 365"},
		{"slack", "Slack"},
		{"aws_console", "AWS Management Console"},
		{"github", "GitHub Enterprise"},
		{"jira", "Atlassian Jira"},
		{"workday", "Workday"},
		{"servicenow", "ServiceNow"},
		{"zoom", "Zoom"},
		{"google_workspace", "Google Workspace"},
	}

	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148",
		"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/120.0.0.0 Mobile Safari/537.36",
		"okta-sdk-java/2.0.0 java/17.0.1 Mac_OS_X/14.0",
		"Okta-Workflows/1.0",
	}

	cities = []struct {
		city      string
		state     string
		country   string
		latitude  float64
		longitude float64
	}{
		{"San Francisco", "California", "United States", 37.7749, -122.4194},
		{"New York", "New York", "United States", 40.7128, -74.0060},
		{"London", "", "United Kingdom", 51.5074, -0.1278},
		{"Tokyo", "", "Japan", 35.6762, 139.6503},
		{"Sydney", "New South Wales", "Australia", -33.8688, 151.2093},
		{"Berlin", "", "Germany", 52.5200, 13.4050},
		{"Moscow", "", "Russia", 55.7558, 37.6173},
		{"Beijing", "", "China", 39.9042, 116.4074},
	}

	reasons = []string{
		"INVALID_CREDENTIALS",
		"LOCKED_OUT",
		"MFA_REQUIRED",
		"PASSWORD_EXPIRED",
		"VERIFICATION_FAILED",
		"NETWORK_ZONE_BLACKLISTED",
		"DEVICE_NOT_REGISTERED",
		"SUSPICIOUS_ACTIVITY",
	}
)

// New creates a new Okta log generator
func New(logger *zap.Logger, workers int, rate time.Duration) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	meter := otel.Meter(meterName)

	oktaLogsGenerated, err := meter.Int64Counter(
		metricLogsGenerated,
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	oktaActiveWorkers, err := meter.Int64Gauge(
		metricWorkersActive,
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	oktaWriteErrors, err := meter.Int64Counter(
		metricWriteErrors,
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	return &Generator{
		logger:            logger,
		workers:           workers,
		rate:              rate,
		stopCh:            make(chan struct{}),
		meter:             meter,
		oktaLogsGenerated: oktaLogsGenerated,
		oktaActiveWorkers: oktaActiveWorkers,
		oktaWriteErrors:   oktaWriteErrors,
	}, nil
}

// Start starts the Okta log generator
func (g *Generator) Start(writer output.Writer) error {
	g.logger.Info("Starting Okta log generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the Okta log generator
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Okta log generator")

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("Okta log generator stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SetCountTracker sets the finite generation count tracker.
func (g *Generator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

func (g *Generator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()

	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID))) // #nosec G404

	g.oktaActiveWorkers.Record(context.Background(), 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", componentName),
				attribute.Int("worker_id", workerID),
			),
		),
	)
	defer g.oktaActiveWorkers.Record(context.Background(), 0,
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
			if g.tracker != nil && !g.tracker.Acquire() {
				select {
				case <-g.stopCh:
					return
				case <-g.tracker.ResumeC():
					continue
				}
			}
			err := g.generateAndWriteLog(r, writer, workerID)
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

func (g *Generator) generateAndWriteLog(r *rand.Rand, writer output.Writer, workerID int) error {
	logRecord, err := g.generateOktaLog(r)
	if err != nil {
		g.recordWriteError(errorTypeUnknown, err)
		return fmt.Errorf("generate Okta log: %w", err)
	}

	g.oktaLogsGenerated.Add(context.Background(), 1,
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

func (g *Generator) generateOktaLog(r *rand.Rand) (output.LogRecord, error) {
	now := time.Now().UTC()
	event := eventTypes[r.Intn(len(eventTypes))]     // #nosec G404
	actor := actors[r.Intn(len(actors))]             // #nosec G404
	app := applications[r.Intn(len(applications))]   // #nosec G404
	location := cities[r.Intn(len(cities))]          // #nosec G404
	userAgent := userAgents[r.Intn(len(userAgents))] // #nosec G404

	// Generate UUIDs
	uuid := generateUUID(r)
	actorID := generateUUID(r)
	sessionID := generateUUID(r)
	requestID := generateRequestID(r)

	// Build the Okta System Log event
	logData := map[string]any{
		"uuid":           uuid,
		"published":      now.Format(time.RFC3339Nano),
		"eventType":      event.eventType,
		"version":        "0",
		"severity":       event.severity,
		"displayMessage": event.displayMsg,
		"actor": map[string]any{
			"id":          actorID,
			"type":        actor.userType,
			"alternateId": actor.login,
			"displayName": actor.displayName,
		},
		"client": map[string]any{
			"userAgent": map[string]any{
				"rawUserAgent": userAgent,
				"os":           "Unknown",
				"browser":      "UNKNOWN",
			},
			"zone":      "null",
			"device":    "Unknown",
			"ipAddress": generateRandomIP(r),
			"geographicalContext": map[string]any{
				"city":       location.city,
				"state":      location.state,
				"country":    location.country,
				"postalCode": fmt.Sprintf("%05d", r.Intn(99999)), // #nosec G404
				"geolocation": map[string]any{
					"lat": location.latitude,
					"lon": location.longitude,
				},
			},
		},
		"outcome": map[string]any{
			"result": event.outcome,
		},
		"target": []map[string]any{
			{
				"id":          generateUUID(r),
				"type":        "AppInstance",
				"alternateId": app.name,
				"displayName": app.label,
			},
		},
		"transaction": map[string]any{
			"type":   "WEB",
			"id":     requestID,
			"detail": map[string]any{},
		},
		"debugContext": map[string]any{
			"debugData": map[string]any{
				"requestId":       requestID,
				"requestUri":      "/api/v1/authn",
				"threatSuspected": fmt.Sprintf("%t", event.severity == "ERROR" || event.severity == "WARN"),
				"url":             fmt.Sprintf("/api/v1/authn?%s", requestID),
			},
		},
		"authenticationContext": map[string]any{
			"authenticationProvider": "OKTA_AUTHENTICATION_PROVIDER",
			"credentialProvider":     "OKTA_CREDENTIAL_PROVIDER",
			"credentialType":         "PASSWORD",
			"externalSessionId":      sessionID,
			"interface":              "Okta End-User Dashboard",
		},
		"securityContext": map[string]any{
			"asNumber": r.Intn(65535), // #nosec G404
			"asOrg":    "example-isp",
			"isp":      "Example ISP",
			"domain":   "example.com",
			"isProxy":  r.Float64() < 0.1, // #nosec G404
		},
		"legacyEventType": event.eventType,
	}

	// Add reason for failures
	if event.outcome == "FAILURE" {
		logData["outcome"].(map[string]any)["reason"] = reasons[r.Intn(len(reasons))] // #nosec G404
	}

	jsonBytes, err := jsonlib.Marshal(logData)
	if err != nil {
		return output.LogRecord{}, fmt.Errorf("marshal Okta log: %w", err)
	}

	return output.LogRecord{
		Message: string(jsonBytes),
		ParseFunc: func(message string) (map[string]any, error) {
			var parsed map[string]any
			if err := jsonlib.Unmarshal([]byte(message), &parsed); err != nil {
				return nil, fmt.Errorf("unmarshal Okta log: %w", err)
			}
			return parsed, nil
		},
		Metadata: output.LogRecordMetadata{
			Timestamp: now,
			Severity:  event.severity,
		},
	}, nil
}

func generateUUID(r *rand.Rand) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		r.Uint32(),                // #nosec G404
		r.Uint32()&0xFFFF,         // #nosec G404
		r.Uint32()&0xFFFF,         // #nosec G404
		r.Uint32()&0xFFFF,         // #nosec G404
		r.Uint64()&0xFFFFFFFFFFFF) // #nosec G404
}

func generateRequestID(r *rand.Rand) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 20)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))] // #nosec G404
	}
	return string(b)
}

func generateRandomIP(r *rand.Rand) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		r.Intn(256), // #nosec G404
		r.Intn(256), // #nosec G404
		r.Intn(256), // #nosec G404
		r.Intn(256)) // #nosec G404
}

func (g *Generator) recordWriteError(errorType string, err error) {
	g.oktaWriteErrors.Add(context.Background(), 1,
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
