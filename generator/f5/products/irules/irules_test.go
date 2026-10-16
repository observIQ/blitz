package irules

import (
	"math/rand"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/generator/f5/catalog"
)

// TestTCLErrorMatchesGolden renders a real iRule runtime error, from Splunk
// Connect for Syslog's F5 fixture (tests/test_f5_bigip.py).
func TestTCLErrorMatchesGolden(t *testing.T) {
	golden := `<131>Jan 17 03:38:00 SV5-F5-5600-2 err tmm[23068]: 01220001:3: TCL error: /Common/stg-Artifactory-iRule <HTTP_REQUEST> - ERR_NOT_SUPPORTED (line 8)     invoked from within "HTTP::method"`
	now := time.Date(2020, 1, 17, 3, 38, 0, 0, time.UTC)
	require.Equal(t, golden, renderTCLError(now, "SV5-F5-5600-2", "tmm", 23068, "stg-Artifactory-iRule", "HTTP_REQUEST", 8, "HTTP::method"))
}

func TestBuildShapes(t *testing.T) {
	dev := catalog.NewDevice(rand.New(rand.NewSource(7)))
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1", Device: dev}
	pid := strconv.Itoa(dev.PID("tmm"))
	logRe := regexp.MustCompile(`^<134>Sep 24 15:04:05 bigip1 info tmm\d?\[` + pid + `\]: Rule /Common/\w+ <[A-Z_]+>: .+$`)
	errRe := regexp.MustCompile(`^<131>Sep 24 15:04:05 bigip1 err tmm\d?\[` + pid + `\]: 01220001:3: TCL error: /Common/\w+ <[A-Z_]+> - ERR_NOT_SUPPORTED \(line \d+\)     invoked from within "[A-Z]+::\w+"$`)
	errs := 0
	for seed := int64(0); seed < 200; seed++ {
		line := build(rand.New(rand.NewSource(seed)), c)
		if errRe.MatchString(line) {
			errs++
			require.Equal(t, "ERROR", catalog.SeverityFromPRI(line))
			continue
		}
		require.Regexp(t, logRe, line)
	}
	require.Positive(t, errs)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("irules")
	require.True(t, ok)
}
