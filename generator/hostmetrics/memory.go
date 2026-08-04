package hostmetrics

import (
	"math/rand"
	"time"

	"github.com/observiq/blitz/output"
)

type memoryScraper struct{}

func (s *memoryScraper) Name() string { return "memory" }

func (s *memoryScraper) Scrape(r *rand.Rand, _ string, resource map[string]any) []output.MetricRecord {
	now := time.Now()
	totalGB := []int64{4, 8, 16, 32, 64}
	total := totalGB[r.Intn(len(totalGB))] * 1024 * 1024 * 1024 // #nosec G404

	usedPct := 0.3 + r.Float64()*0.5 // 30-80% #nosec G404
	used := int64(float64(total) * usedPct)
	free := total - used
	cached := int64(float64(free) * (0.2 + r.Float64()*0.3)) // #nosec G404
	buffered := int64(float64(free) * r.Float64() * 0.1)     // #nosec G404

	return []output.MetricRecord{
		{
			Name: "system.memory.usage", Description: "Memory usage in bytes",
			Unit: "By", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(used),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"state": "used"},
				Resource:   resource,
			},
		},
		{
			Name: "system.memory.usage", Description: "Memory usage in bytes",
			Unit: "By", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(free),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"state": "free"},
				Resource:   resource,
			},
		},
		{
			Name: "system.memory.usage", Description: "Memory usage in bytes",
			Unit: "By", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(cached),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"state": "cached"},
				Resource:   resource,
			},
		},
		{
			Name: "system.memory.usage", Description: "Memory usage in bytes",
			Unit: "By", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(buffered),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"state": "buffered"},
				Resource:   resource,
			},
		},
		{
			Name: "system.memory.utilization", Description: "Memory utilization as a fraction",
			Unit: "1", Type: output.MetricTypeGauge,
			DoubleValue: float64Ptr(usedPct),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{},
				Resource:   resource,
			},
		},
	}
}
