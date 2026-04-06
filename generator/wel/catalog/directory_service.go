package catalog

import (
	"fmt"
	"math/rand"
)

const directoryServiceChannel = "Directory Service"

func init() {
	ntdsProvider := "Microsoft-Windows-ActiveDirectory_DomainService"
	ntdsGUID := "{0e8478c5-3605-4e8c-8497-1e3a77b52070}"

	dsEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{1000, LevelInformation, generateNTDSStarting},
		{1001, LevelInformation, generateNTDSStarted},
		{1002, LevelInformation, generateNTDSStopping},
		{1003, LevelInformation, generateNTDSStopped},
		{1084, LevelInformation, generateNTDSReplication},
		{1173, LevelWarning, generateNTDSReplicationWarning},
		{2089, LevelWarning, generateNTDSBackupWarning},
	}

	for _, ev := range dsEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      directoryServiceChannel,
			Provider:     ntdsProvider,
			ProviderGUID: ntdsGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleDC,
			Generate:     ev.gen,
		})
	}
}

func generateNTDSStarting(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	return nil, "Active Directory Domain Services is starting."
}

func generateNTDSStarted(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	return nil, "Active Directory Domain Services has finished starting."
}

func generateNTDSStopping(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	return nil, "Active Directory Domain Services is shutting down."
}

func generateNTDSStopped(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	return nil, "Active Directory Domain Services has completed shutting down."
}

func generateNTDSReplication(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	partitions := []string{"DC=contoso,DC=com", "CN=Configuration,DC=contoso,DC=com", "CN=Schema,CN=Configuration,DC=contoso,DC=com"}
	partition := partitions[r.Intn(len(partitions))] // #nosec G404
	source := PickHostname(r, opts.Hostnames)
	data := []EventDataField{
		{Name: "DestinationDRA", Value: opts.Computer},
		{Name: "SourceDRA", Value: source},
		{Name: "NamingContext", Value: partition},
	}
	return data, fmt.Sprintf("Active Directory Domain Services successfully replicated the partition %s from source %s.", partition, source)
}

func generateNTDSReplicationWarning(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	source := PickHostname(r, opts.Hostnames)
	data := []EventDataField{
		{Name: "DestinationDRA", Value: opts.Computer},
		{Name: "SourceDRA", Value: source},
		{Name: "ErrorCode", Value: "8456"},
	}
	return data, fmt.Sprintf("Internal event: Active Directory Domain Services encountered an error while replicating from source %s. Error: 8456 (Source is currently rejecting replication requests).", source)
}

func generateNTDSBackupWarning(_ *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "Server", Value: opts.Computer},
		{Name: "Partition", Value: "DC=contoso,DC=com"},
		{Name: "DaysSinceBackup", Value: "45"},
	}
	return data, fmt.Sprintf("Active Directory Domain Services on %s has not been backed up since at least 45 days.", opts.Computer)
}
