package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/internal/config"
)

// TestRunWarnsAboutRemovedSettings pins that the CLI logs the removed
// hostmetrics workers warning at startup. An unparsable TLS CA stops the run
// once outputs are built, after the warning.
func TestRunWarnsAboutRemovedSettings(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	dir := t.TempDir()
	logPath := filepath.Join(dir, "blitz.log")
	caPath := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caPath, []byte("not a certificate"), 0o600))
	path := filepath.Join(dir, "blitz.yaml")
	yaml := fmt.Sprintf(`generator:
  type: hostmetrics
  hostmetrics:
    workers: 2
output:
  type: tcp
  tcp:
    host: 127.0.0.1
    port: 1
    enableTLS: true
    tls:
      ca: [%s]
metrics:
  port: %d
logging:
  type: file
  file:
    path: %s
`, caPath, port, logPath)
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	require.ErrorContains(t, run(nil, nil), "certificate authority")
	logged, err := os.ReadFile(logPath) // #nosec G304 -- test temp file
	require.NoError(t, err)
	require.Contains(t, string(logged), config.HostMetricsWorkersRemoved)
}
