package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPromRWBufferHoldsABatch pins that the buffer cap holds at least one
// batch: a batch larger than the buffer could never fill, so only the batch
// timeout (or Stop, when it is 0) would send. Zero takes each default.
func TestPromRWBufferHoldsABatch(t *testing.T) {
	cfg := PrometheusRemoteWriteOutputConfig{Endpoint: "http://rw.example.com/api/v1/write", BatchSize: 20000}
	require.ErrorContains(t, cfg.Validate(), "max buffered series (10000) must be at least the batch size (20000)")

	cfg = PrometheusRemoteWriteOutputConfig{Endpoint: "http://rw.example.com/api/v1/write", MaxBufferedSeries: 100}
	require.ErrorContains(t, cfg.Validate(), "max buffered series (100) must be at least the batch size (500)")

	cfg = PrometheusRemoteWriteOutputConfig{Endpoint: "http://rw.example.com/api/v1/write", BatchSize: 500, MaxBufferedSeries: 500}
	require.NoError(t, cfg.Validate())
}

// TestPromRWValidateRejectsBadSettings pins each Validate rejection.
func TestPromRWValidateRejectsBadSettings(t *testing.T) {
	ok := "http://rw.example.com/api/v1/write"
	for name, tc := range map[string]struct {
		cfg  PrometheusRemoteWriteOutputConfig
		want string
	}{
		"no endpoint":         {PrometheusRemoteWriteOutputConfig{}, "endpoint cannot be empty"},
		"bad URL":             {PrometheusRemoteWriteOutputConfig{Endpoint: "http://[::1"}, "not a valid URL"},
		"bad scheme":          {PrometheusRemoteWriteOutputConfig{Endpoint: "ftp://rw.example.com/write"}, "scheme must be http or https"},
		"no host":             {PrometheusRemoteWriteOutputConfig{Endpoint: "http:/rw.example.com:9090/api/v1/write"}, "endpoint has no host"},
		"bad version":         {PrometheusRemoteWriteOutputConfig{Endpoint: ok, Version: "3.0"}, "version must be one of"},
		"negative batch":      {PrometheusRemoteWriteOutputConfig{Endpoint: ok, BatchSize: -1}, "batch size cannot be negative"},
		"negative timeout":    {PrometheusRemoteWriteOutputConfig{Endpoint: ok, BatchTimeout: -1}, "batch timeout cannot be negative"},
		"negative buffered":   {PrometheusRemoteWriteOutputConfig{Endpoint: ok, MaxBufferedSeries: -1}, "max buffered series cannot be negative"},
		"negative request to": {PrometheusRemoteWriteOutputConfig{Endpoint: ok, Timeout: -1}, "timeout cannot be negative"},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorContains(t, tc.cfg.Validate(), tc.want)
		})
	}
	require.NoError(t, (&PrometheusRemoteWriteOutputConfig{Endpoint: "https://rw.example.com/api/v1/write", Version: "2.0"}).Validate())
}
