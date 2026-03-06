package syslog

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/observiq/blitz/output/syslog/ident"
	"github.com/observiq/blitz/output/tcp"
	"github.com/observiq/blitz/output/udp"
	"go.uber.org/zap"
)

// RFCMode selects the syslog wire format.
type RFCMode string

const (
	// RFC3164Mode formats messages as RFC 3164.
	RFC3164Mode RFCMode = "3164"
	// RFC5424Mode formats messages as RFC 5424.
	RFC5424Mode RFCMode = "5424"
)

// Transport selects the transport type.
type Transport string

const (
	// TransportTCP sends messages over TCP.
	TransportTCP Transport = "tcp"
	// TransportUDP sends messages over UDP.
	TransportUDP Transport = "udp"
)

// Config controls syslog message formatting and transport behavior.
type Config struct {
	Host string
	Port int

	Transport Transport
	RFC       RFCMode

	Workers int

	// Facility is 0-23
	Facility int

	AppName  string
	Hostname string
	ProcID   string
	MsgID    string

	// UDP-only safety limit. If <= 0, no truncation is attempted.
	// If positive and message exceeds the limit, it will be truncated.
	MaxDatagramBytes int

	// TCP-only TLS configuration. If nil, TLS is disabled.
	TLSConfig *tls.Config
}

// Syslog implements output.Output by formatting records as syslog and delegating transport.
type Syslog struct {
	logger    *zap.Logger
	cfg       Config
	transport output.Output
}

// New creates a Syslog output that delegates to TCP or UDP outputs.
func New(logger *zap.Logger, cfg Config) (*Syslog, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if cfg.Host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535, got %d", cfg.Port)
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.Facility < 0 || cfg.Facility > 23 {
		return nil, fmt.Errorf("facility must be between 0 and 23, got %d", cfg.Facility)
	}
	if cfg.RFC == "" {
		cfg.RFC = RFC5424Mode
	}
	if cfg.Transport == "" {
		cfg.Transport = TransportUDP
	}

	var (
		underlying output.Output
		err        error
	)

	switch strings.ToLower(string(cfg.Transport)) {
	case string(TransportTCP):
		underlying, err = tcp.New(
			logger,
			cfg.Host,
			strconv.Itoa(cfg.Port),
			cfg.Workers,
			cfg.TLSConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("create tcp transport: %w", err)
		}
	case string(TransportUDP):
		underlying, err = udp.New(
			logger,
			cfg.Host,
			strconv.Itoa(cfg.Port),
			cfg.Workers,
		)
		if err != nil {
			return nil, fmt.Errorf("create udp transport: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported transport: %q", cfg.Transport)
	}

	return &Syslog{
		logger:    logger.Named("output-syslog"),
		cfg:       cfg,
		transport: underlying,
	}, nil
}

// newWithTransport is intended for tests.
func newWithTransport(logger *zap.Logger, cfg Config, transport output.Output) *Syslog {
	return &Syslog{
		logger:    logger.Named("output-syslog"),
		cfg:       cfg,
		transport: transport,
	}
}

// Write formats a record as syslog and delegates to the underlying transport.
func (s *Syslog) Write(ctx context.Context, rec output.LogRecord) error {
	formatted, err := s.format(rec)
	if err != nil {
		return err
	}

	// Do not add a newline here. The TCP transport already appends one.
	// UDP transport sends bytes as-is.
	if s.cfg.MaxDatagramBytes > 0 && strings.ToLower(string(s.cfg.Transport)) == string(TransportUDP) {
		if len(formatted) > s.cfg.MaxDatagramBytes {
			formatted = formatted[:s.cfg.MaxDatagramBytes]
		}
	}

	return s.transport.Write(ctx, output.LogRecord{Message: formatted})
}

// SupportedTelemetry returns the telemetry types this output supports.
func (s *Syslog) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}

// Stop delegates to the underlying transport.
func (s *Syslog) Stop(ctx context.Context) error {
	return s.transport.Stop(ctx)
}

func (s *Syslog) format(rec output.LogRecord) (string, error) {
	sev := mapSeverity(rec.Metadata.Severity)
	pri := s.cfg.Facility*8 + sev

	ts := rec.Metadata.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	switch s.cfg.RFC {
	case RFC3164Mode:
		// RFC 3164: <PRI>TIMESTAMP HOSTNAME APP-NAME[PROCID]: MSG
		// TIMESTAMP uses "Mmm dd hh:mm:ss" with no year, usually local time.
		lt := ts.Local()
		// Ensure day is space-padded to width 2 (Jan _2 15:04:05)
		timestamp := lt.Format("Jan _2 15:04:05")
		hostname := s.cfg.Hostname
		if strings.TrimSpace(hostname) == "" {
			hostname = ident.RandomHostname()
		}
		app := s.cfg.AppName
		if strings.TrimSpace(app) == "" {
			app = ident.RandomAppName()
		}
		proc := s.cfg.ProcID
		if strings.TrimSpace(proc) == "" {
			proc = ident.RandomProcID()
		}
		msg := sanitizeMessage(rec.Message)

		var appPart string
		if proc != "" {
			appPart = app + "[" + proc + "]"
		} else {
			appPart = app
		}

		return fmt.Sprintf("<%d>%s %s %s: %s", pri, timestamp, hostname, appPart, msg), nil

	case RFC5424Mode:
		// RFC 5424: <PRI>1 TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [STRUCTURED-DATA] MSG
		// Use RFC3339Nano in UTC per spec recommendation.
		utc := ts.UTC().Format(time.RFC3339Nano)
		hostname := s.cfg.Hostname
		if strings.TrimSpace(hostname) == "" {
			hostname = ident.RandomHostname()
		}
		app := s.cfg.AppName
		if strings.TrimSpace(app) == "" {
			app = ident.RandomAppName()
		}
		proc := s.cfg.ProcID
		if strings.TrimSpace(proc) == "" {
			proc = ident.RandomProcID()
		}
		msgID := s.cfg.MsgID
		if strings.TrimSpace(msgID) == "" {
			msgID = ident.RandomMsgID()
		}
		sd := "-" // no structured data initially
		msg := sanitizeMessage(rec.Message)

		return fmt.Sprintf("<%d>1 %s %s %s %s %s %s %s", pri, utc, hostname, app, proc, msgID, sd, msg), nil
	default:
		return "", fmt.Errorf("unsupported rfc mode: %q", s.cfg.RFC)
	}
}

func mapSeverity(sev string) int {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "emerg", "panic", "emergency":
		return 0
	case "alert":
		return 1
	case "crit", "critical", "fatal":
		return 2
	case "err", "error":
		return 3
	case "warn", "warning":
		return 4
	case "notice":
		return 5
	case "info", "":
		return 6
	case "debug", "trace":
		return 7
	default:
		return 6
	}
}

func sanitizeMessage(s string) string {
	// Syslog messages should not contain CR or LF
	// Replace with space to avoid breaking framing.
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	// RFC 6587 non-transparent framing is assumed for TCP (newline added by tcp output).
	return s
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// Prevent unused import error if future changes remove net usage.
var _ = net.IPv4len

// Ensure Syslog implements output.Output
var _ output.Output = (*Syslog)(nil)
