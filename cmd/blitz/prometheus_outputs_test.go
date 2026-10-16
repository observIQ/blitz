package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// runCountedHostMetrics runs the CLI with a short hostmetrics count into the
// given output block, so run returns once the count is reached.
func runCountedHostMetrics(t *testing.T, outputYAML string) error {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	yaml := fmt.Sprintf("generator:\n  type: hostmetrics\n  count: 5\n  hostmetrics:\n    rate: 1ms\n%smetrics:\n  port: %d\n", outputYAML, port)
	path := filepath.Join(t.TempDir(), "blitz.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path
	return run(nil, nil)
}

func TestRunWritesToPrometheusRemoteWrite(t *testing.T) {
	var posts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	require.NoError(t, runCountedHostMetrics(t, fmt.Sprintf("output:\n  type: prometheus-remote-write\n  prometheus-remote-write:\n    endpoint: %s\n", srv.URL)))
	require.NotZero(t, posts.Load(), "stopping the output flushes the counted points")
}
