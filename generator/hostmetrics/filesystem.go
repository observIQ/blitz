package hostmetrics

import (
	"math/rand"
	"time"

	"github.com/observiq/blitz/output"
)

type filesystemScraper struct{}

func (s *filesystemScraper) Name() string { return "filesystem" }

func (s *filesystemScraper) Scrape(r *rand.Rand, _ string, resource map[string]any) []output.MetricRecord {
	now := time.Now()

	type mount struct {
		device     string
		mountpoint string
		fstype     string
	}

	mounts := []mount{
		{"sda1", "/", "ext4"},
		{"sda2", "/home", "ext4"},
		{"sdb1", "/data", "xfs"},
		{"tmpfs", "/tmp", "tmpfs"},
	}
	m := mounts[r.Intn(len(mounts))] // #nosec G404

	attrs := map[string]string{
		"device":     m.device,
		"mountpoint": m.mountpoint,
		"type":       m.fstype,
	}

	totalGB := int64(r.Intn(900)+100) * 1024 * 1024 * 1024 // 100-1000 GB #nosec G404
	usedPct := 0.1 + r.Float64()*0.7                       // 10-80% #nosec G404
	used := int64(float64(totalGB) * usedPct)
	free := totalGB - used

	return []output.MetricRecord{
		{
			Name: "system.filesystem.usage", Description: "Filesystem usage in bytes",
			Unit: "By", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(used),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: mergeAttrs(attrs, "state", "used"),
				Resource:   resource,
			},
		},
		{
			Name: "system.filesystem.usage", Description: "Filesystem usage in bytes",
			Unit: "By", Type: output.MetricTypeGauge,
			IntValue: int64Ptr(free),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: mergeAttrs(attrs, "state", "free"),
				Resource:   resource,
			},
		},
		{
			Name: "system.filesystem.utilization", Description: "Filesystem utilization as a fraction",
			Unit: "1", Type: output.MetricTypeGauge,
			DoubleValue: float64Ptr(usedPct),
			Metadata: output.MetricPointMetadata{
				Timestamp:  now,
				Attributes: attrs,
				Resource:   resource,
			},
		},
	}
}
