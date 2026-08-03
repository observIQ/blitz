package paloalto

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
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

const componentName = "paloalto"

// Generator produces Palo Alto-style syslog lines.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	consumer embed.LogConsumer

	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
}

// New creates a new Palo Alto generator. The consumer receives each
// generated record as a size-1 batch via ConsumeLogs.
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
	g.logger.Info("Starting Palo Alto generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i)
	}
	return nil
}

// Stop stops the generator.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Palo Alto generator")

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
func (g *Generator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

func (g *Generator) worker(workerID int) {
	defer g.wg.Done()
	g.logger.Debug("Starting worker", zap.Int("worker_id", workerID))

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
			if err := g.generateAndWrite(workerID); err != nil {
				g.logger.Error("Failed to write log", zap.Int("worker_id", workerID), zap.Error(err))
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
		}
	}
}

func (g *Generator) generateAndWrite(_ int) error {
	line := generatePaloAltoLog()

	generator.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logRecord := embed.LogRecord{
		Message: line,
		Metadata: embed.LogRecordMetadata{
			Severity: "INFO",
			Resource: resource.Default(componentName),
		},
	}

	if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{logRecord}); err != nil {
		errorType := "unknown"
		if ctx.Err() == context.DeadlineExceeded {
			errorType = "timeout"
		}
		g.recordWriteError(errorType, err)
		return err
	}
	return nil
}

func (g *Generator) recordWriteError(errorType string, _ error) {
	generator.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
	)
}

// ---- Palo Alto log synthesis ----

func generatePaloAltoLog() string {
	now := time.Now()
	timestamp := now.Format("Jan 02 15:04:05")
	dateTime := now.Format("2006/01/02 15:04:05")
	// Generate an older timestamp for some fields
	oldTime := now.Add(-time.Duration(randInt(1, 365*24)) * time.Hour)
	oldDateTime := oldTime.Format("2006/01/02 15:04:05")

	logTypes := []string{"TRAFFIC", "THREAT", "SYSTEM", "CONFIG"}
	logType := logTypes[randInt(0, len(logTypes)-1)]

	sessionID := generateNumericSessionID()

	switch logType {
	case "SYSTEM":
		return generateSystemLog(timestamp, dateTime, oldDateTime, sessionID)
	case "CONFIG":
		return generateConfigLog(timestamp, dateTime, oldDateTime, sessionID)
	case "TRAFFIC":
		return generateTrafficLog(timestamp, dateTime, sessionID)
	case "THREAT":
		return generateThreatLog(timestamp, dateTime, sessionID)
	default:
		return generateSystemLog(timestamp, dateTime, oldDateTime, sessionID)
	}
}

func generateSystemLog(timestamp, dateTime, oldDateTime, sessionID string) string {
	subtypes := []string{"general", "ras", "vpn", "routing"}
	subtype := subtypes[randInt(0, len(subtypes)-1)]

	messages := []string{
		"Config installed",
		"RASMGR daemon configuration load phase-1 succeeded.",
		"RASMGR daemon configuration load phase-2 succeeded.",
		"IKE daemon configuration load phase-1 succeeded.",
		"IKE daemon configuration load phase-2 succeeded.",
		"Route daemon configuration load phase-1 succeeded.",
		"Route daemon configuration load phase-2 succeeded.",
		"Log type config cleared by user",
	}
	message := messages[randInt(0, len(messages)-1)]

	configNum := randInt(800, 1000)
	return fmt.Sprintf("%s 1,%s,%s,SYSTEM,%s,1,%s,,unknown,,0,0,general,informational,%s,%d,0x0",
		timestamp, dateTime, sessionID, subtype, oldDateTime, message, configNum)
}

func generateConfigLog(timestamp, dateTime, oldDateTime, sessionID string) string {
	actions := []string{"commit", "edit", "set"}
	action := actions[randInt(0, len(actions)-1)]

	users := []string{"admin", "badguy", "operator"}
	user := users[randInt(0, len(users)-1)]

	methods := []string{"Web", "CLI"}
	method := methods[randInt(0, len(methods)-1)]

	statuses := []string{"Submitted", "Succeeded"}
	status := statuses[randInt(0, len(statuses)-1)]

	details := []string{
		"",
		" vsys  vsys1 profiles data-objects  PII",
		" config shared local-user-database user  badguy",
		" config mgt-config users  badguy",
		" vsys  vsys1 profiles url-filtering  monzyspolicy",
	}
	detail := details[randInt(0, len(details)-1)]

	ip := generateRandomIP()
	return fmt.Sprintf("%s 1,%s,%s,CONFIG,0,0,%s,%s,,%s,%s,%s,%s%s,0,0x0",
		timestamp, dateTime, sessionID, oldDateTime, ip, action, user, method, status, detail)
}

func generateTrafficLog(timestamp, dateTime, sessionID string) string {
	actions := []string{"allow", "deny", "drop"}
	action := actions[randInt(0, len(actions)-1)]

	sourceIP := generateRandomIP()
	destIP := generateRandomIP()
	sourcePort := generateRandomPort()
	destPort := generateRandomPort()

	protocols := []string{"tcp", "udp", "icmp"}
	protocol := protocols[randInt(0, len(protocols)-1)]

	bytes := randInt(50, 1000)
	packets := randInt(1, 10)

	seqNum := randInt(100000, 999999)
	seqNum2 := randInt(100000, 999999)
	return fmt.Sprintf("%s 1,%s,%s,TRAFFIC,%s,2049,%s,%s,%s,%s,%s,,,incomplete,vsys1,untrusted,trusted,ethernet1/3,ethernet1/2,log-forwarding-default,%s,%d,1,%s,%s,%s,%s,0x400064,%s,%s,%d,%d,0,4,%d,7,any,0,0x0,0,4,0,aged-out,0,0,0,0,,from-policy,,,0,,0,,N/A,0,0,0,0",
		timestamp, dateTime, sessionID, action, dateTime, sourceIP, destIP, sourceIP, destIP, dateTime, seqNum, sourcePort, destPort, sourcePort, destPort, protocol, action, bytes, packets, seqNum2)
}

func generateThreatLog(timestamp, dateTime, sessionID string) string {
	sourceIP := generateRandomIP()
	destIP := generateRandomIP()
	sourcePort := generateRandomPort()
	destPort := generateRandomPort()

	protocols := []string{"tcp", "udp"}
	protocol := protocols[randInt(0, len(protocols)-1)]

	domains := []string{"example.com", "malicious-site.com", "suspicious-domain.net"}
	domain := domains[randInt(0, len(domains)-1)]

	return fmt.Sprintf("%s 1,%s,%s,THREAT,url,1,%s,%s,%s,%s,%s,RFC1918 to Internet,,,web-browsing,vsys1,Trust,Untrust,ae1.902,ae1.1000,LoggingToPanorama,%s,%d,1,%s,%s,%s,%s,0x408000,%s,alert,\"%s\",(9999),not-defined,informational,client-to-server,%d,0x0,%s,0,text/html",
		timestamp, dateTime, sessionID, dateTime, sourceIP, destIP, sourceIP, destIP, dateTime, randInt(100000, 999999), sourcePort, destPort, sourcePort, destPort, protocol, domain, randInt64(1000000000, 9999999999), sourceIP)
}

func generateRandomIP() string {
	ranges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"34.0.0.0/8",
		"134.0.0.0/8",
		"206.0.0.0/8",
	}

	rangeIndex := randInt(0, len(ranges)-1)
	ipRange := ranges[rangeIndex]

	if strings.Contains(ipRange, "10.") {
		return fmt.Sprintf("10.%d.%d.%d", randInt(0, 255), randInt(0, 255), randInt(1, 254))
	} else if strings.Contains(ipRange, "172.") {
		return fmt.Sprintf("172.%d.%d.%d", randInt(16, 31), randInt(0, 255), randInt(1, 254))
	} else if strings.Contains(ipRange, "192.168.") {
		return fmt.Sprintf("192.168.%d.%d", randInt(0, 255), randInt(1, 254))
	}
	return fmt.Sprintf("%d.%d.%d.%d", randInt(1, 254), randInt(0, 255), randInt(0, 255), randInt(1, 254))
}

func generateRandomPort() string {
	commonPorts := datagen.CommonPorts.All()
	if randInt(0, 10) < 7 {
		return strconv.Itoa(commonPorts[randInt(0, len(commonPorts)-1)])
	}
	return strconv.Itoa(randInt(1024, 65535))
}

func generateRandomSessionID() string {
	bytes := make([]byte, 6)
	_, _ = rand.Read(bytes)
	return strings.ToUpper(hex.EncodeToString(bytes))
}

func generateNumericSessionID() string {
	// Generate a numeric session ID like "01606001116" or "1606001116"
	// Length varies between 9-11 digits
	length := randInt(9, 11)
	var sessionID strings.Builder
	for i := range length {
		// First digit can be 0 for IDs with length > 9, otherwise 1-9
		if i == 0 && length > 9 {
			digit := randInt(0, 9)
			sessionID.WriteString(strconv.Itoa(digit))
		} else {
			digit := randInt(0, 9)
			sessionID.WriteString(strconv.Itoa(digit))
		}
	}
	return sessionID.String()
}

func randInt(min, max int) int {
	delta := max - min + 1
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(delta)))
	return min + int(n.Int64())
}

// randInt64 is the int64 sibling of randInt. Required at call sites
// where min or max exceeds the int range on 32-bit platforms (notably
// linux/arm) — see the 10-digit session-id field in the THREAT log.
func randInt64(min, max int64) int64 {
	delta := max - min + 1
	n, _ := rand.Int(rand.Reader, big.NewInt(delta))
	return min + n.Int64()
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
