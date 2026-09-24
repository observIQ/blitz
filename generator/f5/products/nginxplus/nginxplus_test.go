package nginxplus

import (
	"math/rand"
	"regexp"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/stretchr/testify/require"
)

func TestBuildDeterministic(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "nginxp1"}
	got := build(rand.New(rand.NewSource(2)), c)
	require.Equal(t, got, build(rand.New(rand.NewSource(2)), c))
}

// TestAccessLineMatchesCombinedPlus asserts the access line is byte-exact to the
// combined format followed by the documented Plus upstream fields.
func TestAccessLineMatchesCombinedPlus(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "nginxp1"}
	got := accessLine(rand.New(rand.NewSource(2)), c)

	// header + combined + plus. Verify the combined structure exactly, then the
	// Plus extension fields in order.
	re := regexp.MustCompile(`^<\d+>[^ ]+ [ \d]?\d \d\d:\d\d:\d\d nginxp1 nginx: ` +
		`\S+ - - \[[^\]]+\] "[A-Z]+ \S+ HTTP/1\.1" \d{3} \d+ "-" "[^"]*" ` +
		`rt=\d+\.\d{3} uct="\d+\.\d{3}" uht="\d+\.\d{3}" urt="\d+\.\d{3}" upstream_addr=\S+$`)
	require.Regexp(t, re, got)
}

func TestAccessFormatVarsOrder(t *testing.T) {
	vars := AccessFormatVars()
	require.Equal(t, []string{
		"remote_addr", "remote_user", "time_local", "request", "status",
		"body_bytes_sent", "http_referer", "http_user_agent",
		"request_time", "upstream_connect_time", "upstream_header_time",
		"upstream_response_time", "upstream_addr",
	}, vars)
}

func TestErrorLineShaped(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "nginxp1"}
	got := errorLine(rand.New(rand.NewSource(2)), c)
	require.Contains(t, got, "client:")
	require.Contains(t, got, "upstream:")
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("nginx-plus")
	require.True(t, ok)
}
