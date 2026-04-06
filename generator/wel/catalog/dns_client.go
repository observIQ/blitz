package catalog

import (
	"fmt"
	"math/rand"
)

const dnsClientChannel = "Microsoft-Windows-DNS-Client/Operational"

func init() {
	dnsProvider := "Microsoft-Windows-DNS-Client"
	dnsGUID := "{1c95126e-7eea-49a9-a3fe-a378b03ddb4d}"

	Register(EventDefinition{
		Channel:      dnsClientChannel,
		Provider:     dnsProvider,
		ProviderGUID: dnsGUID,
		EventID:      3006,
		Level:        LevelInformation,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			names := []string{"dc01.contoso.com", "www.contoso.com", "mail.contoso.com", "time.windows.com"}
			name := names[r.Intn(len(names))] // #nosec G404
			data := []EventDataField{
				{Name: "QueryName", Value: name},
				{Name: "QueryType", Value: "1"},
				{Name: "QueryStatus", Value: "0"},
				{Name: "QueryResults", Value: RandomIPv4(r)},
			}
			return data, fmt.Sprintf("DNS query completed for %s with status 0 (NOERROR).", name)
		},
	})

	Register(EventDefinition{
		Channel:      dnsClientChannel,
		Provider:     dnsProvider,
		ProviderGUID: dnsGUID,
		EventID:      3008,
		Level:        LevelWarning,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			names := []string{"unknown-host.contoso.com", "stale-record.contoso.com"}
			name := names[r.Intn(len(names))] // #nosec G404
			data := []EventDataField{
				{Name: "QueryName", Value: name},
				{Name: "QueryType", Value: "1"},
				{Name: "QueryStatus", Value: "9003"},
			}
			return data, fmt.Sprintf("DNS query completed for %s with status 9003 (RCODE_NAME_ERROR).", name)
		},
	})
}
