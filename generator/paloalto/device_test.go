package paloalto

import (
	"context"
	"encoding/csv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/internal/datagen"
)

// deviceFields returns every line's Device Name and firewall Serial Number
// (the first Serial Number; a second one is the user's machine).
func deviceFields(t *testing.T, lines [][]byte) (names, serials []string) {
	t.Helper()
	for _, l := range lines {
		csvPart := string(l)[strings.Index(string(l), " 1,")+1:]
		rec, err := csv.NewReader(strings.NewReader(csvPart)).Read()
		require.NoError(t, err)
		fields := logFieldNames[rec[3]]
		serialSeen := false
		for i, f := range fields {
			switch f {
			case "Device Name":
				names = append(names, rec[i])
			case "Serial Number":
				if !serialSeen {
					serials = append(serials, rec[i])
					serialSeen = true
				}
			}
		}
	}
	return names, serials
}

// TestDeviceFieldsNameOneFirewall pins that every line names the one device
// the generator describes: the environment appliance's hostname and serial,
// or, with no appliance, one fallback device for the generator's lifetime.
func TestDeviceFieldsNameOneFirewall(t *testing.T) {
	run := func(opts ...generator.Option) [][]byte {
		w := newMockWriter()
		g, err := New(zap.NewNop(), 1, time.Second, w, embed.NopTelemetry(), opts...)
		require.NoError(t, err)
		for range 40 {
			require.NoError(t, g.generateAndWrite(context.Background(), 0))
		}
		return w.getWrites()
	}

	fw := &datagen.NetworkSystemIdentity{Hostname: "pa-fw-07", Serial: "013201001234", Vendor: datagen.VendorPaloAlto, Model: "PA-3220"}
	names, serials := deviceFields(t, run(generator.WithAppliance(fw)))
	require.NotEmpty(t, names)
	for _, n := range names {
		require.Equal(t, "pa-fw-07", n)
	}
	for _, s := range serials {
		require.Equal(t, "013201001234", s)
	}

	names, serials = deviceFields(t, run())
	for _, n := range names {
		require.Equal(t, names[0], n, "one fallback device name")
	}
	for _, s := range serials {
		require.Equal(t, serials[0], s, "one fallback device serial")
	}

	// A hand-built appliance missing a field keeps the one it sets and falls
	// back for the other, so no line has an empty device name or serial.
	names, serials = deviceFields(t, run(generator.WithAppliance(&datagen.NetworkSystemIdentity{Hostname: "pa-fw-07"})))
	require.Equal(t, "pa-fw-07", names[0])
	require.Regexp(t, `^[0-9A-F]{15}$`, serials[0])
	names, serials = deviceFields(t, run(generator.WithAppliance(&datagen.NetworkSystemIdentity{Serial: "013201001234"})))
	require.Equal(t, "PA-VM", names[0])
	require.Equal(t, "013201001234", serials[0])
}

// TestUnnamedApplianceResourceNamesTheFallback pins that an appliance without
// a hostname, or no appliance at all, reports the fallback name in host.name,
// matching its lines.
func TestUnnamedApplianceResourceNamesTheFallback(t *testing.T) {
	c := &recordConsumer{}
	g, err := New(zap.NewNop(), 1, time.Second, c, embed.NopTelemetry(),
		generator.WithAppliance(&datagen.NetworkSystemIdentity{Serial: "013201001234", Vendor: datagen.VendorPaloAlto}))
	require.NoError(t, err)
	require.NoError(t, g.generateAndWrite(context.Background(), 0))
	require.Len(t, c.recs, 1)
	require.Equal(t, "PA-VM", c.recs[0].Metadata.Resource["host.name"])
	require.Equal(t, "Palo Alto Networks", c.recs[0].Metadata.Resource["device.manufacturer"])

	// No appliance at all: host.name is the fallback device, not the process host.
	c = &recordConsumer{}
	g, err = New(zap.NewNop(), 1, time.Second, c, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, g.generateAndWrite(context.Background(), 0))
	require.Equal(t, "PA-VM", c.recs[0].Metadata.Resource["host.name"])
}

// TestSyslogHeaderNamesTheFirewall pins the PAN-OS default BSD syslog header
// (Syslog HOSTNAME Format defaults to FQDN): a space-padded RFC 3164
// timestamp, then the firewall's FQDN, then the CSV. Without an environment
// device the header names the fallback PA-VM.
func TestSyslogHeaderNamesTheFirewall(t *testing.T) {
	header := `^(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) [ 1-3][0-9] [0-2][0-9]:[0-5][0-9]:[0-5][0-9] `
	lines := func(opts ...generator.Option) [][]byte {
		w := newMockWriter()
		g, err := New(zap.NewNop(), 1, time.Second, w, embed.NopTelemetry(), opts...)
		require.NoError(t, err)
		for range 5 {
			require.NoError(t, g.generateAndWrite(context.Background(), 0))
		}
		return w.getWrites()
	}
	fw := &datagen.NetworkSystemIdentity{Hostname: "pa-fw-07", FQDN: "pa-fw-07.contoso.local", Serial: "013201001234", Vendor: datagen.VendorPaloAlto}
	for _, l := range lines(generator.WithAppliance(fw)) {
		require.Regexp(t, header+`pa-fw-07\.contoso\.local 1,`, string(l))
	}
	for _, l := range lines() {
		require.Regexp(t, header+`PA-VM 1,`, string(l))
	}
}

// recordConsumer keeps every record it receives.
type recordConsumer struct {
	mu   sync.Mutex
	recs []embed.LogRecord
}

func (c *recordConsumer) ConsumeLogs(_ context.Context, r []embed.LogRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recs = append(c.recs, r...)
	return nil
}

// TestRecordsAreSyslogFramed pins that palo-alto marks its records as
// syslog-framed, since each line carries the firewall's own BSD header, so a
// syslog output sends it as the firewall would.
func TestRecordsAreSyslogFramed(t *testing.T) {
	c := &recordConsumer{}
	g, err := New(zap.NewNop(), 1, time.Second, c, embed.NopTelemetry())
	require.NoError(t, err)
	for range 5 {
		require.NoError(t, g.generateAndWrite(context.Background(), 0))
	}
	require.Len(t, c.recs, 5)
	for _, r := range c.recs {
		require.True(t, r.Metadata.SyslogFramed)
	}
}

// TestUserDomainFollowsTheDevice pins that domain users carry the device's
// domain (the capture's blitz\jdoe on pa-fw-01.blitz.local), not a fixed one.
func TestUserDomainFollowsTheDevice(t *testing.T) {
	w := newMockWriter()
	fw := &datagen.NetworkSystemIdentity{Hostname: "pa-fw-07", FQDN: "pa-fw-07.contoso.local", Serial: "013201001234", Vendor: datagen.VendorPaloAlto}
	g, err := New(zap.NewNop(), 1, time.Second, w, embed.NopTelemetry(), generator.WithAppliance(fw))
	require.NoError(t, err)
	domainUsers := 0
	for range 2000 {
		require.NoError(t, g.generateAndWrite(context.Background(), 0))
	}
	for _, l := range w.getWrites() {
		rec, err := csv.NewReader(strings.NewReader(string(l)[strings.Index(string(l), " 1,")+1:])).Read()
		require.NoError(t, err)
		for i, name := range logFieldNames[rec[3]] {
			switch name {
			case "Source User", "User", "User by Source":
				if d, _, ok := strings.Cut(rec[i], `\`); ok {
					require.Equal(t, "contoso", d, "%s %s", rec[3], name)
					domainUsers++
				}
			}
		}
	}
	require.Positive(t, domainUsers)
}
