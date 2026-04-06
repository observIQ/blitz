package hostmetrics

import (
	"math/rand"
	"time"

	"github.com/observiq/blitz/output"
)

type networkScraper struct{}

func (s *networkScraper) Name() string { return "network" }

func (s *networkScraper) Scrape(r *rand.Rand, _ string, resource map[string]string) []output.MetricRecord {
	now := time.Now()
	ifaces := []string{"eth0", "eth1", "lo", "ens192", "bond0"}
	iface := ifaces[r.Intn(len(ifaces))] // #nosec G404

	attrs := map[string]string{"device": iface}

	recvBytes := int64(r.Intn(1024*1024*500)) + 1024 // #nosec G404
	sentBytes := int64(r.Intn(1024*1024*500)) + 1024 // #nosec G404
	recvPkts := int64(r.Intn(100000)) + 100          // #nosec G404
	sentPkts := int64(r.Intn(100000)) + 100          // #nosec G404
	recvErrs := int64(r.Intn(50))                    // #nosec G404
	sentErrs := int64(r.Intn(50))                    // #nosec G404
	recvDrops := int64(r.Intn(20))                   // #nosec G404
	sentDrops := int64(r.Intn(20))                   // #nosec G404

	return []output.MetricRecord{
		{
			Name: "system.network.io", Description: "Network I/O bytes",
			Unit: "By", Type: output.MetricTypeSum,
			IntValue: int64Ptr(recvBytes), Timestamp: now,
			Attributes: mergeAttrs(attrs, "direction", "receive"), Resource: resource,
		},
		{
			Name: "system.network.io", Description: "Network I/O bytes",
			Unit: "By", Type: output.MetricTypeSum,
			IntValue: int64Ptr(sentBytes), Timestamp: now,
			Attributes: mergeAttrs(attrs, "direction", "transmit"), Resource: resource,
		},
		{
			Name: "system.network.packets", Description: "Network packets",
			Unit: "{packet}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(recvPkts), Timestamp: now,
			Attributes: mergeAttrs(attrs, "direction", "receive"), Resource: resource,
		},
		{
			Name: "system.network.packets", Description: "Network packets",
			Unit: "{packet}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(sentPkts), Timestamp: now,
			Attributes: mergeAttrs(attrs, "direction", "transmit"), Resource: resource,
		},
		{
			Name: "system.network.errors", Description: "Network errors",
			Unit: "{error}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(recvErrs), Timestamp: now,
			Attributes: mergeAttrs(attrs, "direction", "receive"), Resource: resource,
		},
		{
			Name: "system.network.errors", Description: "Network errors",
			Unit: "{error}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(sentErrs), Timestamp: now,
			Attributes: mergeAttrs(attrs, "direction", "transmit"), Resource: resource,
		},
		{
			Name: "system.network.dropped", Description: "Network dropped packets",
			Unit: "{packet}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(recvDrops), Timestamp: now,
			Attributes: mergeAttrs(attrs, "direction", "receive"), Resource: resource,
		},
		{
			Name: "system.network.dropped", Description: "Network dropped packets",
			Unit: "{packet}", Type: output.MetricTypeSum,
			IntValue: int64Ptr(sentDrops), Timestamp: now,
			Attributes: mergeAttrs(attrs, "direction", "transmit"), Resource: resource,
		},
	}
}
