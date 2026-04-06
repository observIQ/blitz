package catalog

import (
	"fmt"
	"math/rand"
)

const dfsReplicationChannel = "DFS Replication"

func init() {
	dfsProvider := "Microsoft-Windows-DFSR"
	dfsGUID := "{2db9a758-1425-47a3-98d0-1a6da95dc0cb}"

	dfsEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4104, LevelInformation, generateDFSRReplicationStarted},
		{4114, LevelWarning, generateDFSRConnectionLost},
		{5002, LevelInformation, generateDFSRGroupProcessed},
		{5008, LevelWarning, generateDFSRStagingQuota},
	}

	for _, ev := range dfsEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      dfsReplicationChannel,
			Provider:     dfsProvider,
			ProviderGUID: dfsGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleDC,
			Generate:     ev.gen,
		})
	}
}

func generateDFSRReplicationStarted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	partner := PickHostname(r, opts.Hostnames)
	data := []EventDataField{
		{Name: "ReplicationGroupName", Value: "Domain System Volume"},
		{Name: "PartnerName", Value: partner},
	}
	return data, fmt.Sprintf("The DFS Replication service started replication with partner %s for replication group Domain System Volume.", partner)
}

func generateDFSRConnectionLost(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	partner := PickHostname(r, opts.Hostnames)
	data := []EventDataField{
		{Name: "ReplicationGroupName", Value: "Domain System Volume"},
		{Name: "PartnerName", Value: partner},
		{Name: "ErrorCode", Value: "9036"},
	}
	return data, fmt.Sprintf("The DFS Replication service detected that the connection with partner %s is down. Error: 9036.", partner)
}

func generateDFSRGroupProcessed(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "ReplicationGroupName", Value: "Domain System Volume"},
		{Name: "FolderName", Value: "SYSVOL Share"},
		{Name: "FilesProcessed", Value: "42"},
	}
	return data, "The DFS Replication service successfully processed 42 updates for replication group Domain System Volume, folder SYSVOL Share."
}

func generateDFSRStagingQuota(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "ReplicationGroupName", Value: "Domain System Volume"},
		{Name: "FolderName", Value: "SYSVOL Share"},
		{Name: "StagingQuota", Value: "4096"},
		{Name: "StagingUsed", Value: "3800"},
	}
	return data, "The DFS Replication service is nearing the staging folder quota. Currently using 3800 MB of 4096 MB quota."
}
