package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

func TestBuildOutputs_SingleIsReturnedUnwrapped(t *testing.T) {
	out, signals, err := buildOutputs(zap.NewNop(), []config.Output{{Type: config.OutputTypeNop}}, embed.NopTelemetry())
	require.NoError(t, err)
	t.Cleanup(func() { _ = out.Stop(context.Background()) })

	_, isMulti := out.(*output.MultiOutput)
	assert.False(t, isMulti, "a single output is used directly, not wrapped")
	require.Len(t, signals, 1)
	assert.Equal(t, "nop", signals[0].Name)
	assert.Equal(t, []telemetry.Type{telemetry.Logs}, signals[0].Signals)
}

func TestBuildOutputs_MultiFansOutAndNamesEachByPosition(t *testing.T) {
	out, signals, err := buildOutputs(zap.NewNop(), []config.Output{
		{Type: config.OutputTypeNop},
		{Type: config.OutputTypeStdout, Stdout: config.StdoutOutputConfig{FlushInterval: config.DefaultStdoutFlushInterval}},
	}, embed.NopTelemetry())
	require.NoError(t, err)
	t.Cleanup(func() { _ = out.Stop(context.Background()) })

	_, isMulti := out.(*output.MultiOutput)
	assert.True(t, isMulti)
	require.Len(t, signals, 2)
	assert.Equal(t, "outputs[0] (nop)", signals[0].Name)
	assert.Equal(t, []telemetry.Type{telemetry.Logs}, signals[0].Signals)
	assert.Equal(t, "outputs[1] (stdout)", signals[1].Name)
	assert.Equal(t, []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}, signals[1].Signals)
}

// stopRecorder is a built output that records whether it was stopped and
// returns err from Stop.
type stopRecorder struct {
	output.Output
	stopped bool
	err     error
}

func (*stopRecorder) SupportedTelemetry() []telemetry.Type { return []telemetry.Type{telemetry.Logs} }

func (s *stopRecorder) Stop(context.Context) error {
	s.stopped = true
	return s.err
}

func TestBuildOutputs_LaterFailureStopsAlreadyBuiltOutputs(t *testing.T) {
	first := &stopRecorder{}
	boom := errors.New("boom")
	orig := buildOneOutput
	t.Cleanup(func() { buildOneOutput = orig })
	buildOneOutput = func(_ *zap.Logger, oc config.Output, _ embed.TelemetrySettings) (output.Output, error) {
		if oc.Type == config.OutputTypeNop {
			return first, nil
		}
		return nil, boom
	}

	_, _, err := buildOutputs(zap.NewNop(), []config.Output{
		{Type: config.OutputTypeNop},
		{Type: config.OutputTypeStdout},
	}, embed.NopTelemetry())
	require.ErrorIs(t, err, boom)
	assert.True(t, first.stopped, "an output built before the failure must be stopped, not leaked")
}

func TestBuildOutputs_LogsAStopFailureDuringCleanup(t *testing.T) {
	first := &stopRecorder{err: errors.New("stuck")}
	orig := buildOneOutput
	t.Cleanup(func() { buildOneOutput = orig })
	buildOneOutput = func(_ *zap.Logger, oc config.Output, _ embed.TelemetrySettings) (output.Output, error) {
		if oc.Type == config.OutputTypeNop {
			return first, nil
		}
		return nil, errors.New("boom")
	}

	core, logs := observer.New(zap.WarnLevel)
	_, _, err := buildOutputs(zap.New(core), []config.Output{
		{Type: config.OutputTypeNop},
		{Type: config.OutputTypeStdout},
	}, embed.NopTelemetry())
	require.Error(t, err)
	assert.Equal(t, 1, logs.FilterMessage("failed to stop output after build failure").Len())
}

// TestRunStopsOutputsWhenStartupFails pins that run stops the outputs it built
// when a later startup step fails (here, signal validation), rather than
// leaving their connections and fan-out goroutines running.
func TestRunStopsOutputsWhenStartupFails(t *testing.T) {
	built := &stopRecorder{}
	orig := buildOneOutput
	t.Cleanup(func() { buildOneOutput = orig })
	buildOneOutput = func(*zap.Logger, config.Output, embed.TelemetrySettings) (output.Output, error) { return built, nil }

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	path := filepath.Join(t.TempDir(), "blitz.yaml")
	yaml := fmt.Sprintf("generator:\n  type: hostmetrics\noutput:\n  type: nop\nmetrics:\n  port: %d\n", port)
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	err = run(nil, nil)
	require.ErrorContains(t, err, "signal compatibility")
	assert.True(t, built.stopped, "outputs built before a startup failure must be stopped")
}

func TestRunLogsAStopFailureAfterStartupFails(t *testing.T) {
	built := &stopRecorder{err: errors.New("stuck")}
	orig := buildOneOutput
	t.Cleanup(func() { buildOneOutput = orig })
	buildOneOutput = func(*zap.Logger, config.Output, embed.TelemetrySettings) (output.Output, error) { return built, nil }

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	dir := t.TempDir()
	logPath := filepath.Join(dir, "blitz.log")
	path := filepath.Join(dir, "blitz.yaml")
	yaml := fmt.Sprintf("generator:\n  type: hostmetrics\noutput:\n  type: nop\nmetrics:\n  port: %d\nlogging:\n  type: file\n  file:\n    path: %s\n", port, logPath)
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	require.ErrorContains(t, run(nil, nil), "signal compatibility")
	logged, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(logged), "failed to stop outputs after startup failure")
}

// TestBuildOutputs_FanOutChildMetricsNameTheirPosition pins that two fan-out
// outputs of one type report their own blitz.output.* metrics: each child's
// meter carries its list position as the "output" scope attribute.
func TestBuildOutputs_FanOutChildMetricsNameTheirPosition(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.NopTelemetry()
	tel.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	out, _, err := buildOutputs(zap.NewNop(), []config.Output{{Type: config.OutputTypeNop}, {Type: config.OutputTypeNop}}, tel)
	require.NoError(t, err)
	require.NoError(t, out.(*output.MultiOutput).WriteLog(context.Background(), output.LogRecord{Message: "x"}))
	require.NoError(t, out.Stop(context.Background()))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var positions []string
	for _, sm := range rm.ScopeMetrics {
		if sm.Scope.Name != "output" {
			continue
		}
		if v, ok := sm.Scope.Attributes.Value("output"); ok {
			positions = append(positions, v.AsString())
		}
	}
	assert.ElementsMatch(t, []string{"outputs[0]", "outputs[1]"}, positions)
}

// TestRunRejectsExplicitSingularOutputEqualToDefault pins that a singular
// output flag or env var next to an outputs: list is rejected even when its
// value equals the default, rather than silently ignored.
func TestRunRejectsExplicitSingularOutputEqualToDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blitz.yaml")
	// hostmetrics into nop fails signal compatibility, so a run that gets past
	// the exclusivity check stops there instead of running.
	require.NoError(t, os.WriteFile(path, []byte("generator:\n  type: hostmetrics\noutputs:\n  - type: nop\n"), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	t.Run("env", func(t *testing.T) {
		t.Setenv("BLITZ_OUTPUT_TYPE", "nop")
		require.ErrorContains(t, run(nil, nil), "set either output or outputs, not both")
	})
	t.Run("flag", func(t *testing.T) {
		f := rootCmd.PersistentFlags().Lookup("output-type")
		require.NoError(t, f.Value.Set("nop"))
		f.Changed = true
		t.Cleanup(func() { _ = f.Value.Set(""); f.Changed = false })
		require.ErrorContains(t, run(nil, nil), "set either output or outputs, not both")
	})
}

// TestRunIgnoresUnprefixedEnvVars pins that the CLI reads only BLITZ_ env
// vars: an unrelated OUTPUTS in the environment neither replaces nor breaks a
// config's outputs.
func TestRunIgnoresUnprefixedEnvVars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blitz.yaml")
	// hostmetrics into nop fails signal compatibility, so a run that gets past
	// config loading stops there instead of running.
	require.NoError(t, os.WriteFile(path, []byte("generator:\n  type: hostmetrics\noutput:\n  type: nop\n"), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path
	t.Setenv("OUTPUTS", "prod")

	require.ErrorContains(t, run(nil, nil), "signal compatibility")
}
