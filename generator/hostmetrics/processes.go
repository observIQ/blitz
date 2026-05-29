package hostmetrics

import (
	"math/rand"
	"time"

	"github.com/observiq/blitz/output"
)

type processesScraper struct{}

func (s *processesScraper) Name() string { return "processes" }

func (s *processesScraper) Scrape(r *rand.Rand, _ string, resource map[string]string) []output.MetricRecord {
	now := time.Now()

	running := int64(r.Intn(20)) + 1    // #nosec G404
	sleeping := int64(r.Intn(200)) + 50 // #nosec G404
	stopped := int64(r.Intn(5))         // #nosec G404
	zombie := int64(r.Intn(3))          // #nosec G404
	total := running + sleeping + stopped + zombie

	return []output.MetricRecord{
		{
			Name: "system.processes.count", Description: "Total process count by state",
			Unit: "{process}", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(running),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"status": "running"},
				Resource:   resource,
			},
		},
		{
			Name: "system.processes.count", Description: "Total process count by state",
			Unit: "{process}", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(sleeping),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"status": "sleeping"},
				Resource:   resource,
			},
		},
		{
			Name: "system.processes.count", Description: "Total process count by state",
			Unit: "{process}", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(stopped),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"status": "stopped"},
				Resource:   resource,
			},
		},
		{
			Name: "system.processes.count", Description: "Total process count by state",
			Unit: "{process}", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(zombie),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{"status": "zombie"},
				Resource:   resource,
			},
		},
		{
			Name: "system.processes.count", Description: "Total process count",
			Unit: "{process}", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(total),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: map[string]string{},
				Resource:   resource,
			},
		},
	}
}
