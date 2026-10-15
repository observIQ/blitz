//go:build !windows

package main

// The scrape test stops run by sending the process SIGTERM, and syscall.Kill
// does not exist on Windows, so it builds on the other platforms only.

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRunServesPrometheusScrape scrapes a running CLI at its configured
// address and metrics path, so the output block must reach the endpoint.
func TestRunServesPrometheusScrape(t *testing.T) {
	addr := freeAddr(t)
	metricsPort := freeAddr(t).(*net.TCPAddr).Port
	yaml := fmt.Sprintf("generator:\n  type: hostmetrics\n  hostmetrics:\n    rate: 10ms\n"+
		"output:\n  type: prometheus-scrape\n  prometheus-scrape:\n    listenAddress: %s\n    metricsPath: /custom\n"+
		"metrics:\n  port: %d\n", addr, metricsPort)
	path := filepath.Join(t.TempDir(), "blitz.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	done := make(chan error, 1)
	go func() { done <- run(nil, nil) }()
	var body string
	require.Eventually(t, func() bool {
		resp, err := http.Get("http://" + addr.String() + "/custom")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		body = string(b)
		return resp.StatusCode == http.StatusOK && strings.Contains(body, "system_")
	}, 10*time.Second, 20*time.Millisecond, "scrape the configured address and path")

	// run stops on SIGTERM, which its own handler catches.
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("run did not stop on SIGTERM")
	}
}

// freeAddr returns a loopback address with a port free at call time.
func freeAddr(t *testing.T) net.Addr {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr()
	require.NoError(t, l.Close())
	return addr
}
