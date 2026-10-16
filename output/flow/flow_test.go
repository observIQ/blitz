package flow

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	nf "github.com/netsampler/goflow2/v2/decoders/netflow"
	gsf "github.com/netsampler/goflow2/v2/decoders/sflow"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/telemetry"
)

func TestConsumeFlowsSendsEncodedPacket(t *testing.T) {
	srv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer srv.Close()
	_, portStr, _ := net.SplitHostPort(srv.LocalAddr().String())

	o, err := New(zap.NewNop(), "127.0.0.1", portStr, ProtocolNetFlowV5, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	defer o.Stop(context.Background())

	rec := embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8),
		SrcPort: 1234, DstPort: 53, Protocol: 17, Packets: 1, Bytes: 60,
	}
	require.NoError(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{rec}))

	buf := make([]byte, 2048)
	require.NoError(t, srv.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, _, err := srv.ReadFromUDP(buf)
	require.NoError(t, err)
	require.Equal(t, uint16(5), binary.BigEndian.Uint16(buf[0:2])) // NetFlow v5 version
	require.Equal(t, 24+48, n)                                     // header + one record
}

// TestSFlowOutputStreamsLargeRecords pins that the sFlow output sends each
// datagram as it is encoded, so a record with many sampled packets never
// materializes all of its samples at once.
func TestSFlowOutputStreamsLargeRecords(t *testing.T) {
	srv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer srv.Close()
	_, portStr, _ := net.SplitHostPort(srv.LocalAddr().String())
	o, err := New(zap.NewNop(), "127.0.0.1", portStr, ProtocolSFlow, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	defer o.Stop(context.Background())

	rec := embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8),
		SrcPort: 1234, DstPort: 53, Protocol: 17, Packets: 200_000, Bytes: 200_000 * 60, SamplingRate: 1024,
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	require.NoError(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{rec}))
	runtime.ReadMemStats(&after)
	// TotalAlloc is process-wide, so the bound leaves room for other
	// goroutines: streaming allocates about 200 KiB, materializing 200,000
	// samples tens of MiB.
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(8<<20))
}

// TestSFlowOutputAgentIsTheExporter pins that an sFlow output with no agentIP
// reports each record's exporter as the agent address.
func TestSFlowOutputAgentIsTheExporter(t *testing.T) {
	srv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer srv.Close()
	_, portStr, _ := net.SplitHostPort(srv.LocalAddr().String())
	o, err := New(zap.NewNop(), "127.0.0.1", portStr, ProtocolSFlow, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	defer o.Stop(context.Background())
	rec := embed.FlowRecord{SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8), Protocol: 17, Packets: 1, Bytes: 60, ExporterIP: net.IPv4(10, 9, 9, 9)}
	require.NoError(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{rec}))
	buf := make([]byte, 2048)
	require.NoError(t, srv.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, _, err = srv.ReadFromUDP(buf)
	require.NoError(t, err)
	require.Equal(t, []byte{10, 9, 9, 9}, buf[8:12], "agent address")
}

// TestSFlowOutputReturnsSendErrors pins that an sFlow send failure is
// returned from ConsumeFlows, not dropped.
func TestSFlowOutputReturnsSendErrors(t *testing.T) {
	o, err := New(zap.NewNop(), "127.0.0.1", "9", ProtocolSFlow, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	require.NoError(t, o.conn.Close())
	rec := embed.FlowRecord{SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8), Protocol: 17, Packets: 1, Bytes: 60}
	require.ErrorContains(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{rec}), "flow output: send")
}

func TestEveryProtocolSendsItsFormat(t *testing.T) {
	version := func(d []byte) uint32 { return uint32(binary.BigEndian.Uint16(d[0:2])) }
	require.Equal(t, uint32(5), version(sendOne(t, ProtocolNetFlowV5, "", testFlow())))
	require.Equal(t, uint32(9), version(sendOne(t, ProtocolNetFlowV9, "", testFlow())))
	require.Equal(t, uint32(10), version(sendOne(t, ProtocolIPFIX, "", testFlow())))
	d := sendOne(t, ProtocolSFlow, "", testFlow())
	var p gsf.Packet
	require.NoError(t, gsf.DecodeMessageVersion(bytes.NewBuffer(d), &p))
	require.Equal(t, uint32(5), p.Version)
	require.Equal(t, net.IPv4(127, 0, 0, 1).To4(), net.IP(p.AgentIP), "default agent address")
}

// TestEmptyProtocolIsDefault: config validation accepts an empty protocol as
// netflow-v9, so the output must too.
func TestEmptyProtocolIsDefault(t *testing.T) {
	require.Equal(t, uint16(9), binary.BigEndian.Uint16(sendOne(t, "", "", testFlow())[0:2]))
}

func TestUnknownProtocolRejected(t *testing.T) {
	_, err := New(zap.NewNop(), "127.0.0.1", "9995", "netflow-v7", "", "", embed.TelemetrySettings{})
	require.ErrorContains(t, err, "unknown protocol")
}

func TestV5RejectsIPv6Flow(t *testing.T) {
	o, err := New(zap.NewNop(), "127.0.0.1", "9995", ProtocolNetFlowV5, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	defer o.Stop(context.Background())
	f := testFlow()
	f.SrcIP, f.DstIP = net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::2")
	require.ErrorContains(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{f}), "IPv6")
}

func TestSupportedTelemetryIsFlows(t *testing.T) {
	o, err := New(zap.NewNop(), "127.0.0.1", "9995", ProtocolIPFIX, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	defer o.Stop(context.Background())
	require.Equal(t, []telemetry.Type{telemetry.Flows}, o.SupportedTelemetry())
}

// TestTemplatePrecedesDataAcrossWorkers: with several generator workers
// calling ConsumeFlows at once, the packet that carries the template must
// reach the collector before any data packet that depends on it.
func TestTemplatePrecedesDataAcrossWorkers(t *testing.T) {
	for trial := 0; trial < 300; trial++ {
		srv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		require.NoError(t, err)
		require.NoError(t, srv.SetReadBuffer(1<<22))
		_, port, _ := net.SplitHostPort(srv.LocalAddr().String())
		o, err := New(zap.NewNop(), "127.0.0.1", port, ProtocolNetFlowV9, "", "", embed.TelemetrySettings{})
		require.NoError(t, err)

		const workers, each = 8, 5
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < each; i++ {
					_ = o.ConsumeFlows(context.Background(), []embed.FlowRecord{testFlow()})
				}
			}()
		}
		wg.Wait()

		ts := nf.CreateTemplateSystem()
		buf := make([]byte, 65535)
		for i := 0; i < workers*each; i++ {
			require.NoError(t, srv.SetReadDeadline(time.Now().Add(2*time.Second)))
			n, _, err := srv.ReadFromUDP(buf)
			require.NoError(t, err)
			var v9 nf.NFv9Packet
			var ipfix nf.IPFIXPacket
			require.NoError(t, nf.DecodeMessageVersion(bytes.NewBuffer(buf[:n]), ts, &v9, &ipfix), "trial %d packet %d", trial, i)
		}
		_ = o.Stop(context.Background())
		_ = srv.Close()
	}
}

func TestNewRejectsBadAddressAndLogger(t *testing.T) {
	_, err := New(nil, "127.0.0.1", "9995", ProtocolIPFIX, "", "", embed.TelemetrySettings{})
	require.Error(t, err)
	_, err = New(zap.NewNop(), "127.0.0.1", "not-a-port", ProtocolIPFIX, "", "", embed.TelemetrySettings{})
	require.ErrorContains(t, err, "dial")
}

func TestConsumeFlowsSendErrorAfterStop(t *testing.T) {
	o, err := New(zap.NewNop(), "127.0.0.1", "9995", ProtocolIPFIX, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	require.NoError(t, o.Stop(context.Background()))
	require.ErrorContains(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{testFlow()}), "send")
}

func TestSFlowAgentIPConfigured(t *testing.T) {
	srv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.LocalAddr().String())
	o, err := New(zap.NewNop(), "127.0.0.1", port, ProtocolSFlow, "", "192.0.2.9", embed.TelemetrySettings{})
	require.NoError(t, err)
	defer o.Stop(context.Background())
	require.NoError(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{testFlow()}))
	buf := make([]byte, 2048)
	require.NoError(t, srv.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, _, err := srv.ReadFromUDP(buf)
	require.NoError(t, err)
	var p gsf.Packet
	require.NoError(t, gsf.DecodeMessageVersion(bytes.NewBuffer(buf[:n]), &p))
	require.Equal(t, net.IPv4(192, 0, 2, 9).To4(), net.IP(p.AgentIP))
}

// TestNewReportsMetricsError pins that New fails, closing its socket, when
// the meter cannot build the output metrics.
func TestNewReportsMetricsError(t *testing.T) {
	_, err := New(zap.NewNop(), "127.0.0.1", "9995", ProtocolNetFlowV9, "", "", embed.TelemetrySettings{MeterProvider: failingMeterProvider{}})
	require.ErrorContains(t, err, "flow output: build metrics")
}

// failingMeterProvider's meter fails the first instrument the output metrics
// build.
type failingMeterProvider struct{ metric.MeterProvider }

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{Meter: metricnoop.NewMeterProvider().Meter("test")}
}

type failingMeter struct{ metric.Meter }

func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, errors.New("instrument error")
}

// A failed send is counted in blitz.output.send_errors, as the other outputs
// count theirs, for both the packet encoders and the sFlow stream. The batch's
// records still count in blitz.output.entries_received, since the output
// received them.
func TestSendErrorsAreCounted(t *testing.T) {
	for _, proto := range []Protocol{ProtocolNetFlowV9, ProtocolSFlow} {
		reader := sdkmetric.NewManualReader()
		o, err := New(zap.NewNop(), "127.0.0.1", "9", proto, "", "", embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))})
		require.NoError(t, err)
		require.NoError(t, o.conn.Close())
		rec := embed.FlowRecord{SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8), Protocol: 17, Packets: 1, Bytes: 60}
		require.Error(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{rec}))
		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(context.Background(), &rm))
		sum := map[string]int64{}
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if d, ok := m.Data.(metricdata.Sum[int64]); ok {
					for _, dp := range d.DataPoints {
						sum[m.Name] += dp.Value
					}
				}
			}
		}
		require.Equal(t, int64(1), sum["blitz.output.send_errors"], proto)
		require.Equal(t, int64(1), sum["blitz.output.entries_received"], proto)
	}
}

// failFirstConn is a net.Conn whose first Write fails and whose later writes
// are kept.
type failFirstConn struct {
	net.Conn
	failed bool
	pkts   [][]byte
}

func (c *failFirstConn) Write(b []byte) (int, error) {
	if !c.failed {
		c.failed = true
		return 0, errors.New("connection refused")
	}
	c.pkts = append(c.pkts, append([]byte(nil), b...))
	return len(b), nil
}

func (c *failFirstConn) Close() error { return nil }

// TestFailedSendResendsTemplates pins that a template lost to a failed send
// goes out again with the next packet, so a collector never gets data it has
// no template for.
func TestFailedSendResendsTemplates(t *testing.T) {
	for _, p := range []Protocol{ProtocolNetFlowV9, ProtocolIPFIX} {
		o, err := New(zap.NewNop(), "127.0.0.1", "9995", p, "", "", embed.TelemetrySettings{})
		require.NoError(t, err)
		require.NoError(t, o.conn.Close())
		conn := &failFirstConn{}
		o.conn = conn

		require.Error(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{testFlow()}), p)
		require.NoError(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{testFlow()}), p)
		require.Len(t, conn.pkts, 1, p)

		// A fresh template system is a collector that saw nothing before.
		var v9 nf.NFv9Packet
		var ipfix nf.IPFIXPacket
		require.NoError(t, nf.DecodeMessageVersion(bytes.NewBuffer(conn.pkts[0]), nf.CreateTemplateSystem(), &v9, &ipfix), p)
	}
}

// TestV5SendFailureNeedsNoTemplates pins that NetFlow v5, which has no
// templates, sends normally after a failed send.
func TestV5SendFailureNeedsNoTemplates(t *testing.T) {
	o, err := New(zap.NewNop(), "127.0.0.1", "9995", ProtocolNetFlowV5, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	require.NoError(t, o.conn.Close())
	conn := &failFirstConn{}
	o.conn = conn

	require.Error(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{testFlow()}))
	require.NoError(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{testFlow()}))
	require.Len(t, conn.pkts, 1)
	require.Equal(t, 24+48, len(conn.pkts[0]))
}

// TestNewRejectsAnInvalidAgentIP pins that New, like config validation,
// refuses an sFlow agentIP that is not an address, instead of silently
// reporting each record's exporter.
func TestNewRejectsAnInvalidAgentIP(t *testing.T) {
	_, err := New(zap.NewNop(), "127.0.0.1", "9995", ProtocolSFlow, "", "10.0.0.300", embed.TelemetrySettings{})
	require.ErrorContains(t, err, "agentIP")
}
