package syslog

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type stubOutput struct {
	lastWrite string
	stopped   bool
}

func (s *stubOutput) WriteLog(ctx context.Context, data output.LogRecord) error {
	s.lastWrite = data.Message
	return nil
}

func (s *stubOutput) Stop(ctx context.Context) error {
	s.stopped = true
	return nil
}

func (s *stubOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}

func TestFormatRFC5424(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := Config{
		Host:      "example.com",
		Port:      514,
		Transport: TransportUDP,
		RFC:       RFC5424Mode,
		Facility:  1,
		AppName:   "app",
		Hostname:  "host",
		ProcID:    "123",
		MsgID:     "id",
	}

	stub := &stubOutput{}
	s := newWithTransport(logger, cfg, stub)

	ts := time.Date(2025, 1, 2, 3, 4, 5, 6, time.UTC)
	err := s.WriteLog(context.Background(), output.LogRecord{
		Message: "hello\nworld",
		Metadata: output.LogRecordMetadata{
			Timestamp: ts,
			Severity:  "info",
		},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	wantPrefix := "<14>1 2025-01-02T03:04:05.000000006Z host app 123 id - hello world"
	if got := stub.lastWrite; got != wantPrefix {
		t.Fatalf("unexpected formatted message:\n got: %q\nwant: %q", got, wantPrefix)
	}
}

func TestFormatRFC3164(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := Config{
		Host:      "example.com",
		Port:      514,
		Transport: TransportUDP,
		RFC:       RFC3164Mode,
		Facility:  1,
		AppName:   "app",
		Hostname:  "host",
		ProcID:    "123",
	}

	stub := &stubOutput{}
	s := newWithTransport(logger, cfg, stub)

	// Fixed local time: use UTC but RFC3164 uses local; force Location to Local for stable test
	loc := time.FixedZone("Local", 0)
	origLocal := time.Local
	time.Local = loc
	defer func() { time.Local = origLocal }()
	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, loc)
	err := s.WriteLog(context.Background(), output.LogRecord{
		Message: "hello",
		Metadata: output.LogRecordMetadata{
			Timestamp: ts,
			Severity:  "warn",
		},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Jan  2 03:04:05 (space-padded day)
	want := "<12>Jan  2 03:04:05 host app[123]: hello"
	if stub.lastWrite != want {
		t.Fatalf("unexpected formatted message:\n got: %q\nwant: %q", stub.lastWrite, want)
	}
}

func TestUDPLengthTruncation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := Config{
		Host:             "example.com",
		Port:             514,
		Transport:        TransportUDP,
		RFC:              RFC5424Mode,
		Facility:         1,
		AppName:          "app",
		Hostname:         "host",
		MaxDatagramBytes: 32,
	}

	stub := &stubOutput{}
	s := newWithTransport(logger, cfg, stub)

	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	err := s.WriteLog(context.Background(), output.LogRecord{
		Message: strings.Repeat("a", 200),
		Metadata: output.LogRecordMetadata{
			Timestamp: ts,
			Severity:  "info",
		},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if len(stub.lastWrite) != 32 {
		t.Fatalf("expected truncation to 32 bytes, got %d", len(stub.lastWrite))
	}
}

func TestStopDelegates(t *testing.T) {
	logger := zap.NewNop()
	cfg := Config{Host: "h", Port: 514, Transport: TransportUDP, RFC: RFC5424Mode}
	stub := &stubOutput{}
	s := newWithTransport(logger, cfg, stub)
	_ = s.Stop(context.Background())
	if !stub.stopped {
		t.Fatalf("expected stop to delegate to transport")
	}
}

func TestRandomDefaultsRFC5424(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := Config{
		Host:      "example.com",
		Port:      514,
		Transport: TransportUDP,
		RFC:       RFC5424Mode,
		Facility:  1,
		// Intentionally leave Hostname, AppName, ProcID, MsgID empty
	}
	stub := &stubOutput{}
	s := newWithTransport(logger, cfg, stub)

	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	err := s.WriteLog(context.Background(), output.LogRecord{
		Message: "hello",
		Metadata: output.LogRecordMetadata{
			Timestamp: ts,
			Severity:  "info",
		},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	parts := strings.SplitN(stub.lastWrite, " ", 8)
	if len(parts) < 7 {
		t.Fatalf("unexpected formatted message: %q", stub.lastWrite)
	}
	hostname := parts[2]
	app := parts[3]
	proc := parts[4]
	msgid := parts[5]
	if hostname == "-" || app == "-" || proc == "-" || msgid == "-" {
		t.Fatalf("expected random defaults, got hostname=%q app=%q proc=%q msgid=%q", hostname, app, proc, msgid)
	}
}

func TestRandomDefaultsRFC3164(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := Config{
		Host:      "example.com",
		Port:      514,
		Transport: TransportUDP,
		RFC:       RFC3164Mode,
		Facility:  1,
		// Intentionally leave Hostname, AppName, ProcID empty
	}
	stub := &stubOutput{}
	s := newWithTransport(logger, cfg, stub)

	loc := time.FixedZone("Local", 0)
	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, loc)
	err := s.WriteLog(context.Background(), output.LogRecord{
		Message: "hello",
		Metadata: output.LogRecordMetadata{
			Timestamp: ts,
			Severity:  "warn",
		},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if strings.Contains(stub.lastWrite, " - ") {
		t.Fatalf("unexpected dash defaults in RFC3164 formatted message: %q", stub.lastWrite)
	}
	if !strings.Contains(stub.lastWrite, ": ") {
		t.Fatalf("expected RFC3164 tag separator ': ' in message: %q", stub.lastWrite)
	}
}

// TestHostnameFollowsTheRecord pins that with no configured hostname the
// header names the record's host (its host.name), in both RFC modes, so one
// generator's records all come from one host.
func TestHostnameFollowsTheRecord(t *testing.T) {
	for _, mode := range []RFCMode{RFC3164Mode, RFC5424Mode} {
		stub := &stubOutput{}
		s := newWithTransport(zaptest.NewLogger(t), Config{Transport: TransportUDP, RFC: mode, Facility: 1, AppName: "app", ProcID: "1", MsgID: "id"}, stub)
		for i := 0; i < 3; i++ {
			if err := s.WriteLog(context.Background(), output.LogRecord{Message: "hi", Metadata: output.LogRecordMetadata{
				Timestamp: time.Now(), Resource: map[string]any{"host.name": "web-01"}}}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stub.lastWrite, " web-01 ") {
				t.Fatalf("%s header does not name the record's host: %q", mode, stub.lastWrite)
			}
		}
	}
}

// TestSyslogFramedRecordsKeepTheirFrame pins that a record whose message is
// already a syslog line (a device's own BSD header) is sent as written: a
// PRI is added when the line has none, and nothing else.
func TestSyslogFramedRecordsKeepTheirFrame(t *testing.T) {
	stub := &stubOutput{}
	s := newWithTransport(zaptest.NewLogger(t), Config{Transport: TransportUDP, RFC: RFC5424Mode, Facility: 1, Hostname: "host", AppName: "app"}, stub)
	write := func(msg string) string {
		if err := s.WriteLog(context.Background(), output.LogRecord{Message: msg, Metadata: output.LogRecordMetadata{
			Timestamp: time.Now(), Severity: "info", SyslogFramed: true}}); err != nil {
			t.Fatal(err)
		}
		return stub.lastWrite
	}
	if got := write("Sep 30 16:18:12 pa-fw-01.blitz.local 1,2026/09/30 16:18:12,013201001234,SYSTEM"); got != "<14>Sep 30 16:18:12 pa-fw-01.blitz.local 1,2026/09/30 16:18:12,013201001234,SYSTEM" {
		t.Fatalf("frame without PRI: %q", got)
	}
	if got := write("<134>Sep 29 10:10:46 bigip1 info tmm1[34750]: GET /"); got != "<134>Sep 29 10:10:46 bigip1 info tmm1[34750]: GET /" {
		t.Fatalf("frame with PRI: %q", got)
	}
}

// TestHasPRIFollowsRFC5424 pins the PRI grammar: 1-3 digits, 0-191, with no
// leading zero (except PRI 0 itself) and no sign.
func TestHasPRIFollowsRFC5424(t *testing.T) {
	for msg, want := range map[string]bool{
		"<0>x": true, "<14>x": true, "<191>x": true,
		"<01>x": false, "<+1>x": false, "<-1>x": false, "<192>x": false, "<>x": false, "14>x": false, "<1a>x": false,
	} {
		if got := hasPRI(msg); got != want {
			t.Errorf("hasPRI(%q) = %v, want %v", msg, got, want)
		}
	}
}
