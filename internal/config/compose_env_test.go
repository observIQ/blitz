package config

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestComposeEnvKeysBind fails when the telemetry-generator compose file sets a
// BLITZ_* variable that no override binds, since blitz silently ignores it.
func TestComposeEnvKeysBind(t *testing.T) {
	raw, err := os.ReadFile("../../docker/docker-compose.telemetry-generator.yml")
	require.NoError(t, err)

	bound := make(map[string]bool)
	for _, o := range DefaultOverrides() {
		bound[o.Env] = true
	}

	var unbound []string
	for _, m := range regexp.MustCompile(`(?m)^\s+(BLITZ_[A-Z0-9_]+):`).FindAllSubmatch(raw, -1) {
		if key := string(m[1]); !bound[key] {
			unbound = append(unbound, key)
		}
	}
	require.Empty(t, unbound, "compose env keys that bind no override")
}
