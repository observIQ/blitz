package wel

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/wel/catalog"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// mockConsumer implements embed.LogConsumer for tests.
type mockConsumer struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockConsumer) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range records {
		m.messages = append(m.messages, r.Message)
	}
	return nil
}

func (m *mockConsumer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *mockConsumer) Messages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func TestNewGenerator(t *testing.T) {
	logger := zap.NewNop()

	t.Run("valid config", func(t *testing.T) {
		gen, err := New(Config{
			Logger:    logger,
			Workers:   1,
			Rate:      100 * time.Millisecond,
			Computer:  "TESTPC",
			Domain:    "TESTDOMAIN",
			Role:      catalog.RoleMember,
			Usernames: []string{"jsmith"},
			IPs:       []string{"10.0.0.1"},
			Hostnames: []string{"WS01"},
			Consumer:  &mockConsumer{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gen == nil {
			t.Fatal("generator should not be nil")
		}
	})

	t.Run("nil logger", func(t *testing.T) {
		_, err := New(Config{Workers: 1, Consumer: &mockConsumer{}})
		if err == nil {
			t.Error("expected error for nil logger")
		}
	})

	t.Run("nil consumer", func(t *testing.T) {
		_, err := New(Config{Logger: logger, Workers: 1})
		if err == nil {
			t.Error("expected error for nil consumer")
		}
	})

	t.Run("zero workers", func(t *testing.T) {
		_, err := New(Config{Logger: logger, Workers: 0, Consumer: &mockConsumer{}})
		if err == nil {
			t.Error("expected error for zero workers")
		}
	})

	t.Run("default role", func(t *testing.T) {
		gen, err := New(Config{
			Logger:    logger,
			Workers:   1,
			Rate:      time.Second,
			Usernames: []string{"test"},
			Consumer:  &mockConsumer{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gen.role != catalog.RoleMember {
			t.Errorf("expected default role member, got %s", gen.role)
		}
	})

	t.Run("invalid channel filter", func(t *testing.T) {
		_, err := New(Config{
			Logger:    logger,
			Workers:   1,
			Rate:      time.Second,
			Channels:  []string{"NonExistentChannel"},
			Usernames: []string{"test"},
			Consumer:  &mockConsumer{},
		})
		if err == nil {
			t.Error("expected error for invalid channel filter")
		}
	})
}

func TestGeneratorStartStop(t *testing.T) {
	logger := zap.NewNop()
	consumer := &mockConsumer{}
	gen, err := New(Config{
		Logger:    logger,
		Workers:   2,
		Rate:      10 * time.Millisecond,
		Computer:  "TESTPC",
		Domain:    "CONTOSO",
		Role:      catalog.RoleWorkstation,
		Usernames: []string{"jsmith", "admin"},
		IPs:       []string{"10.0.0.1"},
		Hostnames: []string{"WS01"},
		Consumer:  consumer,
	})
	require.NoError(t, err)

	require.NoError(t, gen.Start(context.Background()))

	// Wait for the workers to land at least one record. A fixed sleep
	// would have flake risk under a slow scheduler.
	require.Eventually(t, func() bool {
		return consumer.Count() > 0
	}, 2*time.Second, 5*time.Millisecond, "expected at least one event before Stop")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(ctx))

	// Verify output is valid XML-like
	for _, msg := range consumer.Messages() {
		if !strings.Contains(msg, "<Event xmlns=") {
			t.Errorf("expected XML event output, got: %s", msg[:min(len(msg), 100)])
		}
	}
}

// TestGeneratorHonorsFiniteCount confirms the WEL generator stops at the
// configured record budget: with a count tracker set, generation completes the
// tracker (Done fires) rather than running unbounded (PIPE-1111).
func TestGeneratorHonorsFiniteCount(t *testing.T) {
	logger := zap.NewNop()
	consumer := &mockConsumer{}
	gen, err := New(Config{
		Logger:   logger,
		Workers:  2,
		Rate:     5 * time.Millisecond,
		Computer: "TESTPC",
		Domain:   "CONTOSO",
		Role:     catalog.RoleWorkstation,
		Consumer: consumer,
	})
	require.NoError(t, err)

	tracker := count.NewTracker(5)
	gen.SetCountTracker(tracker)

	require.NoError(t, gen.Start(context.Background()))

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("WEL generator should honor the finite count and complete the tracker")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(ctx))

	require.GreaterOrEqual(t, tracker.Emitted(), int64(5), "tracker should record at least the requested count")
	require.Positive(t, consumer.Count(), "consumer should have received records")
}

// TestGeneratorResumesAfterCountReset covers the idle-and-resume path: once the
// budget is exhausted the worker idles (emitting nothing), and a tracker Reset
// unblocks it through the channel Acquire returns, so generation continues
// (PIPE-1111).
func TestGeneratorResumesAfterCountReset(t *testing.T) {
	logger := zap.NewNop()
	consumer := &mockConsumer{}
	gen, err := New(Config{
		Logger:   logger,
		Workers:  1,
		Rate:     5 * time.Millisecond,
		Computer: "TESTPC",
		Domain:   "CONTOSO",
		Role:     catalog.RoleWorkstation,
		Consumer: consumer,
	})
	require.NoError(t, err)

	tracker := count.NewTracker(3)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))

	// The budget completes, then generation holds at the budget: the worker is
	// idle, waiting on the channel Acquire returns, for the whole window.
	require.Eventually(t, func() bool { return consumer.Count() >= 3 }, 5*time.Second, 5*time.Millisecond)
	require.Never(t, func() bool { return consumer.Count() > 3 }, 150*time.Millisecond, 10*time.Millisecond,
		"generation must not exceed the budget while idle")

	// Reset re-opens the budget and unblocks the idle worker.
	tracker.Reset()
	require.Eventually(t, func() bool { return consumer.Count() > 3 }, 5*time.Second, 10*time.Millisecond,
		"generation should resume after the tracker is reset")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(ctx))
}

func TestGeneratorSupportedTelemetry(t *testing.T) {
	logger := zap.NewNop()
	gen, err := New(Config{
		Logger:    logger,
		Workers:   1,
		Rate:      time.Second,
		Usernames: []string{"test"},
		Consumer:  &mockConsumer{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	types := gen.SupportedTelemetry()
	if len(types) != 1 {
		t.Fatalf("expected 1 telemetry type, got %d", len(types))
	}
	if types[0] != telemetry.Logs {
		t.Errorf("expected Logs telemetry type, got %v", types[0])
	}
}

func TestIdentityResolvesOneHostname(t *testing.T) {
	newGen := func(computer string, id *datagen.SystemIdentity) *Generator {
		gen, err := New(Config{
			Logger:    zap.NewNop(),
			Workers:   1,
			Rate:      time.Second,
			Computer:  computer,
			Usernames: []string{"test"},
			Consumer:  &mockConsumer{},
			Identity:  id,
		})
		require.NoError(t, err)
		return gen
	}
	id := &datagen.SystemIdentity{Hostname: "IDENTITY-HOST", OSInfo: datagen.OSInfo{Type: datagen.OSWindows}}

	cases := []struct {
		name, computer string
		id             *datagen.SystemIdentity
		want           string
	}{
		{"identity", "", id, "IDENTITY-HOST"},
		{"explicit over identity", "CFG-HOST", id, "CFG-HOST"},
		{"explicit, no identity", "CFG-HOST", nil, "CFG-HOST"},
		{"fallback", "", nil, "BLITZ-WEL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen := newGen(tc.computer, tc.id)
			require.Equal(t, tc.want, gen.static.Record()["host.name"])
			require.Equal(t, tc.want, gen.static.Record()["wel.computer"])
			require.Equal(t, tc.want, gen.opts.Computer)
			if tc.id != nil {
				require.Equal(t, "windows", gen.static.Record()["os.type"])
			}
		})
	}
	require.Equal(t, "IDENTITY-HOST", id.Hostname, "shared identity must not be mutated")
}

// TestComputerAndDomainFollowDomainMembership pins real Windows behavior: a
// domain-joined host writes its FQDN in <Computer> and its NetBIOS domain as
// the account domain, while a workgroup host writes its bare name and
// WORKGROUP. Payload fields keep the short computer name.
func TestComputerAndDomainFollowDomainMembership(t *testing.T) {
	joined := &datagen.SystemIdentity{Hostname: "WIN-01", Domain: "contoso.local", OSInfo: datagen.OSInfo{Type: datagen.OSWindows}}
	workgroup := &datagen.SystemIdentity{Hostname: "WIN-02", OSInfo: datagen.OSInfo{Type: datagen.OSWindows}}

	cases := []struct {
		name, computer, domain string
		id                     *datagen.SystemIdentity
		wantXML, wantShort     string
		wantDomain             string
	}{
		{name: "domain-joined", id: joined, wantXML: "WIN-01.contoso.local", wantShort: "WIN-01", wantDomain: "CONTOSO"},
		{name: "explicit computer, joined", computer: "PC-9", id: joined, wantXML: "PC-9.contoso.local", wantShort: "PC-9", wantDomain: "CONTOSO"},
		// An explicit domain is the DNS domain the host is joined to, so it
		// drives both <Computer> and the NetBIOS account domain.
		{name: "explicit domain wins", domain: "fabrikam.com", id: joined, wantXML: "WIN-01.fabrikam.com", wantShort: "WIN-01", wantDomain: "FABRIKAM"},
		{name: "explicit workgroup", domain: "WORKGROUP", id: joined, wantXML: "WIN-01", wantShort: "WIN-01", wantDomain: "WORKGROUP"},
		{name: "workgroup", id: workgroup, wantXML: "WIN-02", wantShort: "WIN-02", wantDomain: "WORKGROUP"},
		{name: "no identity", wantXML: "BLITZ-WEL", wantShort: "BLITZ-WEL", wantDomain: "WORKGROUP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cons := &mockConsumer{}
			gen, err := New(Config{
				Logger:    zap.NewNop(),
				Workers:   1,
				Rate:      time.Second,
				Computer:  tc.computer,
				Domain:    tc.domain,
				Usernames: []string{"test"},
				Consumer:  cons,
				Identity:  tc.id,
			})
			require.NoError(t, err)
			require.NoError(t, gen.generateAndWrite(rand.New(rand.NewSource(1))))

			require.Len(t, cons.messages, 1)
			assert.Contains(t, cons.messages[0], "<Computer>"+tc.wantXML+"</Computer>")
			assert.Equal(t, tc.wantShort, gen.opts.Computer)
			assert.Equal(t, tc.wantShort, gen.static.Record()["host.name"])
			assert.Equal(t, tc.wantDomain, gen.opts.DomainName)
			assert.Equal(t, tc.wantDomain, gen.static.Record()["wel.domain"])
		})
	}
}

// TestRoleFollowsIdentity pins that an unset role takes the event pool of the
// host the generator lands on, so a DC emits DC events and a workstation
// workstation events; a set role always wins.
func TestRoleFollowsIdentity(t *testing.T) {
	host := func(role datagen.SystemRole) *datagen.SystemIdentity {
		return &datagen.SystemIdentity{Hostname: "H", Role: role, OSInfo: datagen.OSInfo{Type: datagen.OSWindows}}
	}
	cases := []struct {
		name string
		set  catalog.MachineRole
		id   *datagen.SystemIdentity
		want catalog.MachineRole
	}{
		{"dc host", "", host(datagen.RoleDC), catalog.RoleDC},
		{"server host", "", host(datagen.RoleServer), catalog.RoleMember},
		{"workstation host", "", host(datagen.RoleWorkstation), catalog.RoleWorkstation},
		{"no identity", "", nil, catalog.RoleMember},
		{"set role wins", catalog.RoleWorkstation, host(datagen.RoleDC), catalog.RoleWorkstation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen, err := New(Config{
				Logger:    zap.NewNop(),
				Workers:   1,
				Rate:      time.Second,
				Role:      tc.set,
				Usernames: []string{"test"},
				Consumer:  &mockConsumer{},
				Identity:  tc.id,
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, gen.role)
			require.Equal(t, string(tc.want), gen.static.Record()["wel.role"])
		})
	}
}

// TestSingleLabelDomainWarns pins that a domain with no dot, such as a NetBIOS
// name from a config written for earlier releases, is still read as the DNS
// domain but logged at Warn, since <Computer> then ends in that bare label.
func TestSingleLabelDomainWarns(t *testing.T) {
	for _, tc := range []struct {
		domain string
		warns  bool
	}{{"CONTOSO", true}, {"contoso.local", false}, {"WORKGROUP", false}, {"", false}} {
		core, logs := observer.New(zap.WarnLevel)
		_, err := New(Config{
			Logger:    zap.New(core),
			Workers:   1,
			Rate:      time.Second,
			Domain:    tc.domain,
			Usernames: []string{"test"},
			Consumer:  &mockConsumer{},
		})
		require.NoError(t, err)
		assert.Equal(t, tc.warns, logs.Len() == 1, "domain %q", tc.domain)
	}
}

// flakyConsumer rejects its first failFirst calls, then accepts every one.
type flakyConsumer struct {
	mockConsumer
	failFirst int
	calls     int
}

func (f *flakyConsumer) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
	f.mu.Lock()
	f.calls++
	failed := f.calls <= f.failFirst
	f.mu.Unlock()
	if failed {
		return errors.New("rejected")
	}
	return f.mockConsumer.ConsumeLogs(ctx, records)
}

// TestGeneratorFailedWriteReleasesPermit pins that a rejected write gives its
// permit back, so the count ends on delivered records.
func TestGeneratorFailedWriteReleasesPermit(t *testing.T) {
	consumer := &flakyConsumer{failFirst: 2}
	gen, err := New(Config{
		Logger:   zap.NewNop(),
		Workers:  1,
		Rate:     time.Millisecond,
		Computer: "TESTPC",
		Domain:   "CONTOSO",
		Role:     catalog.RoleWorkstation,
		Consumer: consumer,
	})
	require.NoError(t, err)
	tracker := count.NewTracker(3)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	t.Cleanup(func() { _ = gen.Stop(context.Background()) })

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("count never completed")
	}
	require.Equal(t, 3, consumer.Count(), "every counted record was delivered")
}
