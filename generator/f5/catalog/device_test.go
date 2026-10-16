package catalog

import (
	"math/rand"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeviceStableIdentity(t *testing.T) {
	d := NewDevice(rand.New(rand.NewSource(3)))
	require.NotNil(t, net.ParseIP(d.MgmtIP))
	for _, daemon := range []string{"tmm", "mcpd", "apmd", "httpd", "audit-service", "nginx"} {
		p := d.PID(daemon)
		require.Positive(t, p, daemon)
		require.Equal(t, p, d.PID(daemon), "pid must be stable for %s", daemon)
	}
	require.Equal(t, d, NewDevice(rand.New(rand.NewSource(3))), "same seed, same device")
	require.Panics(t, func() { d.PID("not-a-daemon") })
}

func TestCtxStatePerKey(t *testing.T) {
	c := &Ctx{}
	n := 0
	get := func() *int { return c.State("k", func() any { n++; v := 0; return &v }).(*int) }
	*get() = 7
	require.Equal(t, 7, *get())
	require.Equal(t, 1, n, "init runs once per key")
}

// TestFullNamePrefersTheFQDN pins that FullName is the FQDN when the host has
// one and its hostname otherwise.
func TestFullNamePrefersTheFQDN(t *testing.T) {
	require.Equal(t, "f5-lb-01.blitz.local", (&Ctx{Hostname: "f5-lb-01", FQDN: "f5-lb-01.blitz.local"}).FullName())
	require.Equal(t, "bigip1", (&Ctx{Hostname: "bigip1"}).FullName())
}
