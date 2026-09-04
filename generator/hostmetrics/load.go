package hostmetrics

import (
	"math/rand"
	"time"

	"github.com/observiq/blitz/output"
)

type loadScraper struct{}

func (s *loadScraper) Name() string { return "load" }

func (s *loadScraper) Scrape(r *rand.Rand, _ string, resource map[string]any) []output.MetricRecord {
	now := time.Now()

	// Load averages: 1m > 5m > 15m (typical pattern)
	load1 := 0.1 + r.Float64()*4.0            // #nosec G404
	load5 := load1 * (0.7 + r.Float64()*0.2)  // #nosec G404
	load15 := load5 * (0.7 + r.Float64()*0.2) // #nosec G404

	return []output.MetricRecord{
		{
			Name: "system.cpu.load_average.1m", Description: "1-minute load average",
			Unit: "1", Type: output.MetricTypeGauge,
			DoubleValue: float64Ptr(load1),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{},
				Resource:   resource,
			},
		},
		{
			Name: "system.cpu.load_average.5m", Description: "5-minute load average",
			Unit: "1", Type: output.MetricTypeGauge,
			DoubleValue: float64Ptr(load5),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{},
				Resource:   resource,
			},
		},
		{
			Name: "system.cpu.load_average.15m", Description: "15-minute load average",
			Unit: "1", Type: output.MetricTypeGauge,
			DoubleValue: float64Ptr(load15),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{},
				Resource:   resource,
			},
		},
	}
}
