package promscrape

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestOutput(t *testing.T) *Output {
	t.Helper()
	o, err := New("127.0.0.1:0", "/metrics", false, 5*time.Minute, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	return o
}

func TestWriteLogUnsupported(t *testing.T) {
	o := newTestOutput(t)
	err := o.Write(context.Background(), output.LogRecord{})
	require.ErrorIs(t, err, output.ErrUnsupportedTelemetryType)
}

func TestSupportedTelemetry(t *testing.T) {
	o := newTestOutput(t)
	require.Equal(t, []telemetry.Type{telemetry.Metrics}, o.SupportedTelemetry())
}

func TestScrapeServesExposition(t *testing.T) {
	o := newTestOutput(t)
	v := 0.5
	err := o.WriteMetric(context.Background(), output.MetricRecord{
		Name:        "system_cpu_utilization",
		Type:        embed.MetricTypeGauge,
		DoubleValue: &v,
	})
	require.NoError(t, err)

	body := scrape(t, o)
	require.Contains(t, body, "# TYPE system_cpu_utilization gauge")
	require.Contains(t, body, "system_cpu_utilization 0.5")
}

func TestBindFailureSurfaces(t *testing.T) {
	o := newTestOutput(t)
	_, err := New(o.Addr(), "/metrics", false, 5*time.Minute, embed.TelemetrySettings{}, zap.NewNop())
	require.Error(t, err)
}

func scrape(t *testing.T, o *Output) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+o.Addr()+"/metrics", nil)
	require.NoError(t, err)
	var resp *http.Response
	require.Eventually(t, func() bool {
		resp, err = http.DefaultClient.Do(req)
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

// Resource becomes job/instance on every series plus one target_info series,
// however many points share the target.
func TestScrapeTargetInfoAndLabels(t *testing.T) {
	o := newTestOutput(t)
	res := map[string]any{"host.name": "athena", "telemetry.source": "hostmetrics", "os.type": "linux"}
	for _, state := range []string{"used", "free"} {
		v := 1.0
		require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
			Name:        "system.memory.usage",
			Type:        embed.MetricTypeGauge,
			DoubleValue: &v,
			Metadata: output.MetricPointMetadata{
				Attributes: map[string]string{"state": state},
				Resource:   res,
			},
		}))
	}

	body := scrape(t, o)
	require.Contains(t, body, `system_memory_usage{instance="athena",job="hostmetrics",state="used"} 1`)
	require.Equal(t, 1, strings.Count(body, "\ntarget_info{"))
	require.Contains(t, body, `target_info{host_name="athena",instance="athena",job="hostmetrics",os_type="linux",telemetry_source="hostmetrics"} 1`)
}
