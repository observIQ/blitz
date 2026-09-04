package hostmetrics

import (
	"math/rand"
	"time"

	"github.com/observiq/blitz/output"
)

type diskScraper struct{}

func (s *diskScraper) Name() string { return "disk" }

func (s *diskScraper) Scrape(r *rand.Rand, _ string, resource map[string]any) []output.MetricRecord {
	now := time.Now()
	devices := []string{"sda", "sdb", "nvme0n1"}
	device := devices[r.Intn(len(devices))] // #nosec G404

	attrs := map[string]string{"device": device}

	readBytes := int64(r.Intn(1024*1024*100)) + 1024*1024  // #nosec G404
	writeBytes := int64(r.Intn(1024*1024*100)) + 1024*1024 // #nosec G404
	readOps := int64(r.Intn(10000)) + 100                  // #nosec G404
	writeOps := int64(r.Intn(10000)) + 100                 // #nosec G404
	readTime := r.Float64() * 10.0                         // #nosec G404
	writeTime := r.Float64() * 10.0                        // #nosec G404
	ioTime := r.Float64() * 5.0                            // #nosec G404

	return []output.MetricRecord{
		{
			Name: "system.disk.io", Description: "Disk I/O bytes",
			Unit: "By", Type: output.MetricTypeSum,
			IntValue: int64Ptr(readBytes),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: mergeAttrs(attrs, "direction", "read"),
				Resource:   resource,
			},
		},
		{
			Name: "system.disk.io", Description: "Disk I/O bytes",
			Unit: "By", Type: output.MetricTypeSum,
			IntValue: int64Ptr(writeBytes),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: mergeAttrs(attrs, "direction", "write"),
				Resource:   resource,
			},
		},
		{
			Name: "system.disk.operations", Description: "Disk operations",
			Unit: "{operation}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(readOps),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: mergeAttrs(attrs, "direction", "read"),
				Resource:   resource,
			},
		},
		{
			Name: "system.disk.operations", Description: "Disk operations",
			Unit: "{operation}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(writeOps),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: mergeAttrs(attrs, "direction", "write"),
				Resource:   resource,
			},
		},
		{
			Name: "system.disk.operation_time", Description: "Time spent in disk operations",
			Unit: "s", Type: output.MetricTypeSum,
			DoubleValue: float64Ptr(readTime),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: mergeAttrs(attrs, "direction", "read"),
				Resource:   resource,
			},
		},
		{
			Name: "system.disk.operation_time", Description: "Time spent in disk operations",
			Unit: "s", Type: output.MetricTypeSum,
			DoubleValue: float64Ptr(writeTime),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: mergeAttrs(attrs, "direction", "write"),
				Resource:   resource,
			},
		},
		{
			Name: "system.disk.io_time", Description: "Time disk spent activated",
			Unit: "s", Type: output.MetricTypeSum,
			DoubleValue: float64Ptr(ioTime),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: attrs,
				Resource:   resource,
			},
		},
	}
}

func mergeAttrs(base map[string]string, key, value string) map[string]string {
	merged := make(map[string]string, len(base)+1)
	for k, v := range base {
		merged[k] = v
	}
	merged[key] = value
	return merged
}
