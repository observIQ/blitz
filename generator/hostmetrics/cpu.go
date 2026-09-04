package hostmetrics

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/observiq/blitz/output"
)

type cpuScraper struct{}

func (s *cpuScraper) Name() string { return "cpu" }

func (s *cpuScraper) Scrape(r *rand.Rand, hostname string, resource map[string]any) []output.MetricRecord {
	now := time.Now()
	numCPUs := r.Intn(16) + 1 // #nosec G404

	var records []output.MetricRecord
	states := []string{"user", "system", "idle", "iowait", "steal"}

	for cpu := range numCPUs {
		// Generate usage that sums roughly to 100
		remaining := 100.0
		for i, state := range states {
			var val float64
			if i == len(states)-1 {
				val = remaining
			} else {
				val = r.Float64() * remaining * 0.5 // #nosec G404
				remaining -= val
			}

			attrs := map[string]string{
				"cpu":   cpuID(cpu),
				"state": state,
			}
			records = append(records, output.MetricRecord{
				Name:        "system.cpu.time",
				Description: "Seconds each CPU spent in each mode",
				Unit:        "s",
				Type:        output.MetricTypeSum,
				DoubleValue: float64Ptr(val),
				Metadata: output.MetricPointMetadata{
					Timestamp:  now,
					Attributes: attrs,
					Resource:   resource,
				},
			})
		}
	}

	// system.cpu.utilization (aggregate)
	utilization := 20.0 + r.Float64()*60.0 // 20-80% #nosec G404
	records = append(records, output.MetricRecord{
		Name:        "system.cpu.utilization",
		Description: "CPU utilization as a fraction",
		Unit:        "1",
		Type:        output.MetricTypeGauge,
		DoubleValue: float64Ptr(utilization / 100.0),
		Metadata: output.MetricPointMetadata{
			Timestamp:  now,
			Attributes: map[string]string{},
			Resource:   resource,
		},
	})

	return records
}

func cpuID(n int) string {
	return "cpu" + strconv.Itoa(n)
}

func float64Ptr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64       { return &v }
