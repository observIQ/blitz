package f5

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

// TestProductsNameTheirOwnHost pins the F5 host split: BIG-IP modules run on
// the environment's F5 appliance, NGINX Plus and App Protect on an NGINX
// server host, and F5OS on its platform (appliance-1). Each line names its
// host, and the record's host.name agrees with the line.
func TestProductsNameTheirOwnHost(t *testing.T) {
	bigip := &datagen.NetworkSystemIdentity{Hostname: "f5-lb-01", Vendor: datagen.VendorF5, Model: "BIG-IP i5800",
		ManagementInterface: &datagen.NetworkInterface{IPv4: "10.9.9.9"}}
	web := &datagen.SystemIdentity{Hostname: "web-01", OSInfo: datagen.OSInfo{Type: datagen.OSLinux}}

	cases := map[string]string{
		"asm": "f5-lb-01", "afm": "f5-lb-01", "apm": "f5-lb-01", "dns": "f5-lb-01", "audit": "f5-lb-01", "irules": "f5-lb-01",
		"nginx-plus": "web-01", "nginx-app-protect": "web-01", "f5os": "appliance-1",
	}
	for product, host := range cases {
		t.Run(product, func(t *testing.T) {
			cc := &captureConsumer{}
			g, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Millisecond, Seed: 1, EnabledProducts: []string{product}},
				cc, embed.TelemetrySettings{}, generator.WithAppliance(bigip), generator.WithIdentity(web))
			require.NoError(t, err)
			require.NoError(t, g.Start(context.Background()))
			require.Eventually(t, func() bool { return cc.count() >= 5 }, 5*time.Second, time.Millisecond)
			require.NoError(t, g.Stop(context.Background()))
			cc.mu.Lock()
			defer cc.mu.Unlock()
			for _, rec := range cc.recs[:5] {
				require.Contains(t, rec.Message, " "+host+" ", "line names its host")
				require.Equal(t, host, rec.Metadata.Resource["host.name"])
				for other := range map[string]bool{"f5-lb-01": true, "web-01": true, "appliance-1": true} {
					if other != host {
						require.False(t, strings.Contains(rec.Message, other), "line names the wrong host %q", other)
					}
				}
			}
		})
	}
}

// TestExplicitHostnameReplacesApplianceName pins that any set hostname, even
// the default's own value, replaces the environment appliance's name.
func TestExplicitHostnameReplacesApplianceName(t *testing.T) {
	bigip := &datagen.NetworkSystemIdentity{Hostname: "f5-lb-01", Vendor: datagen.VendorF5}
	for _, name := range []string{"bigip1", "lb-prod-01"} {
		cfg := DefaultConfig()
		cfg.Hostname = name
		g, err := New(zap.NewNop(), cfg, &captureConsumer{}, embed.TelemetrySettings{}, generator.WithAppliance(bigip))
		require.NoError(t, err)
		require.Equal(t, name, g.hosts[catalog.HostBIGIP].name)
	}
	g, err := New(zap.NewNop(), DefaultConfig(), &captureConsumer{}, embed.TelemetrySettings{}, generator.WithAppliance(bigip))
	require.NoError(t, err)
	require.Equal(t, "f5-lb-01", g.hosts[catalog.HostBIGIP].name, "an unset hostname takes the appliance's")
}

// TestUnnamedApplianceFallsBackToTheDefaultName pins that an appliance
// without a hostname names the BIG-IP unit bigip1, in the line and host.name,
// and keeps the appliance's other fields.
func TestUnnamedApplianceFallsBackToTheDefaultName(t *testing.T) {
	bigip := &datagen.NetworkSystemIdentity{Vendor: datagen.VendorF5, Model: "BIG-IP i5800"}
	cc := &captureConsumer{}
	g, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Millisecond, Seed: 1, EnabledProducts: []string{"asm"}},
		cc, embed.TelemetrySettings{}, generator.WithAppliance(bigip))
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cc.count() >= 1 }, 5*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
	cc.mu.Lock()
	defer cc.mu.Unlock()
	rec := cc.recs[0]
	require.Contains(t, rec.Message, " bigip1 ")
	require.Contains(t, rec.Message, `unit_hostname="bigip1"`)
	require.Equal(t, "bigip1", rec.Metadata.Resource["host.name"])
	require.Equal(t, "BIG-IP i5800", rec.Metadata.Resource["device.model.name"])
}

// TestRecordsAreSyslogFramed pins that every F5 product marks its records as
// syslog-framed, since each line carries the device's own <PRI> and header,
// so a syslog output sends it as the device would.
func TestRecordsAreSyslogFramed(t *testing.T) {
	cc := &captureConsumer{}
	g, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Millisecond, Seed: 1}, cc, embed.TelemetrySettings{})
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cc.count() >= 50 }, 5*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
	cc.mu.Lock()
	defer cc.mu.Unlock()
	for _, rec := range cc.recs {
		require.True(t, rec.Metadata.SyslogFramed, "%q", rec.Message)
	}
}

// TestUnnamedIdentityFallsBackToTheProcessHost pins that an NGINX identity
// without a hostname is treated as no identity (the process host), rather
// than panicking on a missing host.name.
func TestUnnamedIdentityFallsBackToTheProcessHost(t *testing.T) {
	var hosts map[catalog.Host]host
	require.NotPanics(t, func() {
		hosts = resolveHosts("", false, generator.Options{Identity: &datagen.SystemIdentity{}})
	})
	nginx := hosts[catalog.HostNGINX]
	require.NotEmpty(t, nginx.name)
	require.Equal(t, nginx.name, nginx.static.Record()["host.name"])
}

// TestBIGIPLinesCarryTheFQDN pins where a BIG-IP writes its fully qualified
// name: the DNS query log body (the header keeps the short name) and both the
// AFM header and bigip_hostname, as the vendor samples show. An explicit
// hostname has no domain, so it is used in both places.
func TestBIGIPLinesCarryTheFQDN(t *testing.T) {
	bigip := &datagen.NetworkSystemIdentity{Hostname: "f5-lb-01", FQDN: "f5-lb-01.blitz.local", Vendor: datagen.VendorF5,
		ManagementInterface: &datagen.NetworkInterface{IPv4: "10.9.9.9"}}
	lines := func(t *testing.T, cfg Config, product string) []string {
		cfg.Workers, cfg.Rate, cfg.Seed, cfg.EnabledProducts = 1, time.Millisecond, 1, []string{product}
		cc := &captureConsumer{}
		g, err := New(zap.NewNop(), cfg, cc, embed.TelemetrySettings{}, generator.WithAppliance(bigip))
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cc.count() >= 5 }, 5*time.Second, time.Millisecond)
		require.NoError(t, g.Stop(context.Background()))
		return cc.messages(5)
	}
	for _, l := range lines(t, Config{}, "dns") {
		require.Contains(t, l, " f5-lb-01 info tmm", "the header keeps the short name")
		require.Contains(t, l, " f5-lb-01.blitz.local qid ", "the body carries the FQDN")
	}
	for _, l := range lines(t, Config{}, "afm") {
		require.Contains(t, l, " f5-lb-01.blitz.local tmm[")
		require.Contains(t, l, `,"f5-lb-01.blitz.local",`)
	}
	for _, l := range lines(t, Config{Hostname: "lb-prod-01"}, "afm") {
		require.Contains(t, l, `,"lb-prod-01",`)
		require.NotContains(t, l, "blitz.local")
	}
}
