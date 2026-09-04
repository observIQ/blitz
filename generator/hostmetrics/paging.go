package hostmetrics

import (
	"math/rand"
	"time"

	"github.com/observiq/blitz/output"
)

type pagingScraper struct{}

func (s *pagingScraper) Name() string { return "paging" }

func (s *pagingScraper) Scrape(r *rand.Rand, _ string, resource map[string]any) []output.MetricRecord {
	now := time.Now()

	swapTotal := int64(r.Intn(8)+1) * 1024 * 1024 * 1024 // 1-8 GB #nosec G404
	swapUsedPct := r.Float64() * 0.3                     // 0-30% #nosec G404
	swapUsed := int64(float64(swapTotal) * swapUsedPct)
	swapFree := swapTotal - swapUsed

	pageIn := int64(r.Intn(1000))  // #nosec G404
	pageOut := int64(r.Intn(1000)) // #nosec G404
	faults := int64(r.Intn(5000))  // #nosec G404

	return []output.MetricRecord{
		{
			Name: "system.paging.usage", Description: "Swap usage in bytes",
			Unit: "By", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(swapUsed),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"state": "used"},
				Resource:   resource,
			},
		},
		{
			Name: "system.paging.usage", Description: "Swap usage in bytes",
			Unit: "By", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(swapFree),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"state": "free"},
				Resource:   resource,
			},
		},
		{
			Name: "system.paging.utilization", Description: "Swap utilization as a fraction",
			Unit: "1", Type: output.MetricTypeGauge,
			DoubleValue: float64Ptr(swapUsedPct),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{},
				Resource:   resource,
			},
		},
		{
			Name: "system.paging.operations", Description: "Paging operations",
			Unit: "{operation}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(pageIn),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"direction": "page_in"},
				Resource:   resource,
			},
		},
		{
			Name: "system.paging.operations", Description: "Paging operations",
			Unit: "{operation}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(pageOut),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"direction": "page_out"},
				Resource:   resource,
			},
		},
		{
			Name: "system.paging.faults", Description: "Page faults",
			Unit: "{fault}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(faults),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{},
				Resource:   resource,
			},
		},
	}
}
