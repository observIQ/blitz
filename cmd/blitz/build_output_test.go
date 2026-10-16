package main

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	blitzconfig "github.com/observiq/blitz/config"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/config"
)

// defaultOutput returns typ's output config with every default applied, as
// the loader resolves it.
func defaultOutput(t *testing.T, typ config.OutputType) config.Output {
	t.Helper()
	required := map[config.OutputType]string{
		config.OutputTypeTCP:                   "  tcp:\n    host: 127.0.0.1\n    port: 9\n",
		config.OutputTypeUDP:                   "  udp:\n    host: 127.0.0.1\n    port: 9\n",
		config.OutputTypeSyslog:                "  syslog:\n    host: 127.0.0.1\n    port: 9\n",
		config.OutputTypeFile:                  "  file:\n    path: " + filepath.Join(t.TempDir(), "out.log") + "\n",
		config.OutputTypeHEC:                   "  hec:\n    host: 127.0.0.1\n    port: 9\n    token: t\n",
		config.OutputTypeFlow:                  "  flow:\n    host: 127.0.0.1\n    port: 9\n",
		config.OutputTypePrometheusRemoteWrite: "  prometheus-remote-write:\n    endpoint: http://127.0.0.1:9/api/v1/write\n",
		config.OutputTypePrometheusScrape:      "  prometheus-scrape:\n    listenAddress: 127.0.0.1:0\n",
	}
	cfg, err := blitzconfig.Load([]byte(fmt.Sprintf("output:\n  type: %s\n%s", typ, required[typ])), blitzconfig.LoadOpts{})
	require.NoError(t, err)
	return cfg.Output
}

// outputTypes returns every OutputType constant declared in internal/config,
// so a new output type fails these tests until it has cases.
func outputTypes(t *testing.T) []config.OutputType {
	t.Helper()
	files, err := filepath.Glob("../../internal/config/*.go")
	require.NoError(t, err)
	var types []config.OutputType
	for _, f := range files {
		file, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "OutputType" {
					continue
				}
				for _, v := range vs.Values {
					s, err := strconv.Unquote(v.(*ast.BasicLit).Value)
					require.NoError(t, err)
					types = append(types, config.OutputType(s))
				}
			}
		}
	}
	require.NotEmpty(t, types)
	return types
}

// TestBuildOutput_EveryType builds each output type from its defaults and
// with TLS enabled, so every case's options and TLS conversion run.
func TestBuildOutput_EveryType(t *testing.T) {
	missingCA := config.TLS{CertificateAuthority: []string{filepath.Join(t.TempDir(), "missing-ca.pem")}}
	held, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })
	busy := held.Addr().String()
	cases := []struct {
		name    string
		typ     config.OutputType
		mutate  func(o *config.Output)
		wantErr string
	}{
		{name: "nop", typ: config.OutputTypeNop},
		{name: "stdout", typ: config.OutputTypeStdout},
		{name: "tcp", typ: config.OutputTypeTCP},
		{name: "tcp tls", typ: config.OutputTypeTCP, mutate: func(o *config.Output) { o.TCP.EnableTLS = true }},
		{name: "tcp bad tls", typ: config.OutputTypeTCP, mutate: func(o *config.Output) { o.TCP.EnableTLS, o.TCP.TLS = true, missingCA }, wantErr: "certificate authority"},
		{name: "udp", typ: config.OutputTypeUDP},
		{name: "syslog", typ: config.OutputTypeSyslog},
		{name: "syslog tls", typ: config.OutputTypeSyslog, mutate: func(o *config.Output) {
			o.Syslog.Transport, o.Syslog.EnableTLS = config.SyslogTransportTCP, true
		}},
		{name: "syslog bad tls", typ: config.OutputTypeSyslog, mutate: func(o *config.Output) {
			o.Syslog.Transport, o.Syslog.EnableTLS, o.Syslog.TLS = config.SyslogTransportTCP, true, missingCA
		}, wantErr: "certificate authority"},
		{name: "otlp-grpc", typ: config.OutputTypeOTLPGrpc, mutate: func(o *config.Output) {
			o.OTLPGrpc.BatchTimeout, o.OTLPGrpc.RequestTimeout = 1, 1
			o.OTLPGrpc.MaxQueueSize, o.OTLPGrpc.MaxExportBatchSize = 10, 5
		}},
		{name: "otlp-grpc tls", typ: config.OutputTypeOTLPGrpc, mutate: func(o *config.Output) { o.OTLPGrpc.EnableTLS, o.OTLPGrpc.Insecure = true, false }},
		{name: "otlp-grpc bad tls", typ: config.OutputTypeOTLPGrpc, mutate: func(o *config.Output) {
			o.OTLPGrpc.EnableTLS, o.OTLPGrpc.Insecure, o.OTLPGrpc.TLS = true, false, missingCA
		}, wantErr: "certificate authority"},
		{name: "file", typ: config.OutputTypeFile},
		{name: "hec", typ: config.OutputTypeHEC},
		{name: "hec tls", typ: config.OutputTypeHEC, mutate: func(o *config.Output) { o.HEC.EnableTLS = true }},
		{name: "hec bad tls", typ: config.OutputTypeHEC, mutate: func(o *config.Output) { o.HEC.EnableTLS, o.HEC.TLS = true, missingCA }, wantErr: "certificate authority"},
		{name: "flow", typ: config.OutputTypeFlow},
		{name: "flow bad agent ip", typ: config.OutputTypeFlow, mutate: func(o *config.Output) {
			o.Flow.Protocol, o.Flow.AgentIP = "sflow", "not-an-ip"
		}, wantErr: "agent"},
		{name: "prometheus-remote-write", typ: config.OutputTypePrometheusRemoteWrite},
		{name: "prometheus-remote-write bad endpoint", typ: config.OutputTypePrometheusRemoteWrite, mutate: func(o *config.Output) {
			o.PrometheusRemoteWrite.Endpoint = "ftp://127.0.0.1/write"
		}, wantErr: "endpoint"},
		{name: "prometheus-scrape", typ: config.OutputTypePrometheusScrape},
		{name: "prometheus-scrape busy address", typ: config.OutputTypePrometheusScrape, mutate: func(o *config.Output) {
			o.PrometheusScrape.ListenAddress = busy
		}, wantErr: "listen"},
	}
	covered := map[config.OutputType]bool{}
	for _, tc := range cases {
		covered[tc.typ] = true
	}
	for _, typ := range outputTypes(t) {
		require.True(t, covered[typ], "output type %s has no case", typ)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := defaultOutput(t, tc.typ)
			if tc.mutate != nil {
				tc.mutate(&o)
			}
			out, err := buildOutput(zap.NewNop(), o, embed.NopTelemetry())
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, out.Stop(context.Background()))
		})
	}
	_, err = buildOutput(zap.NewNop(), config.Output{Type: "bogus"}, embed.NopTelemetry())
	require.ErrorContains(t, err, "invalid output type")
}

// TestBuildOutput_ConstructorErrors pins that each type returns its
// constructor's error, here a meter that cannot build the output metrics.
// The Prometheus outputs keep running without their self-metrics when those
// cannot be built, so they are the exception (their own tests pin that).
func TestBuildOutput_ConstructorErrors(t *testing.T) {
	bestEffortMetrics := map[config.OutputType]bool{config.OutputTypePrometheusRemoteWrite: true, config.OutputTypePrometheusScrape: true}
	tel := embed.TelemetrySettings{MeterProvider: failingMeterProvider{}}
	for _, typ := range outputTypes(t) {
		out, err := buildOutput(zap.NewNop(), defaultOutput(t, typ), tel)
		if bestEffortMetrics[typ] {
			require.NoError(t, err, typ)
			require.NoError(t, out.Stop(context.Background()), typ)
			continue
		}
		require.Error(t, err, typ)
	}
}

// failingMeterProvider's meter fails every instrument the output metrics
// build first.
type failingMeterProvider struct{ metric.MeterProvider }

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{Meter: metricnoop.NewMeterProvider().Meter("test")}
}

type failingMeter struct{ metric.Meter }

func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, errors.New("instrument error")
}

func (failingMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, errors.New("instrument error")
}

func (failingMeter) Float64Histogram(string, ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return nil, errors.New("instrument error")
}
