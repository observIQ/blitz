package catalog

import (
	"fmt"
	"math/rand"
)

const dnsServerChannel = "DNS Server"

func init() {
	dnsProvider := "Microsoft-Windows-DNS-Server-Service"
	dnsGUID := "{71a551f5-c893-4849-886b-b5ec8502641e}"

	dnsEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{2, LevelWarning, generateDNSServerZoneLoadFailed},
		{4, LevelInformation, generateDNSServerZoneLoaded},
		{150, LevelWarning, generateDNSServerRecursionFailed},
		{501, LevelInformation, generateDNSServerForwarderQuery},
		{7062, LevelWarning, generateDNSServerNoForwarder},
	}

	for _, ev := range dnsEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      dnsServerChannel,
			Provider:     dnsProvider,
			ProviderGUID: dnsGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleDC,
			Generate:     ev.gen,
		})
	}
}

func generateDNSServerZoneLoadFailed(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	zones := []string{"contoso.com", "10.in-addr.arpa", "_msdcs.contoso.com"}
	zone := zones[r.Intn(len(zones))] // #nosec G404
	data := []EventDataField{
		{Name: "ZoneName", Value: zone},
		{Name: "ErrorCode", Value: "9711"},
	}
	return data, fmt.Sprintf("The DNS server could not load zone %s. Error: 9711.", zone)
}

func generateDNSServerZoneLoaded(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	zones := []string{"contoso.com", "10.in-addr.arpa", "_msdcs.contoso.com"}
	zone := zones[r.Intn(len(zones))] // #nosec G404
	data := []EventDataField{
		{Name: "ZoneName", Value: zone},
		{Name: "RecordCount", Value: fmt.Sprintf("%d", r.Intn(1000)+100)}, // #nosec G404
	}
	return data, fmt.Sprintf("The DNS server has loaded zone %s with %s records.", zone, data[1].Value)
}

func generateDNSServerRecursionFailed(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	names := []string{"external.example.com", "cdn.example.net", "api.thirdparty.com"}
	name := names[r.Intn(len(names))] // #nosec G404
	data := []EventDataField{
		{Name: "QueryName", Value: name},
		{Name: "QueryType", Value: "A"},
		{Name: "ErrorCode", Value: "9002"},
	}
	return data, fmt.Sprintf("The DNS server was unable to resolve the query for %s. Error: 9002 (DNS server failure).", name)
}

func generateDNSServerForwarderQuery(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	forwarders := []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"}
	fwd := forwarders[r.Intn(len(forwarders))] // #nosec G404
	data := []EventDataField{
		{Name: "ForwarderIP", Value: fwd},
		{Name: "QueryName", Value: "www.example.com"},
	}
	return data, fmt.Sprintf("The DNS server forwarded query for www.example.com to forwarder %s.", fwd)
}

func generateDNSServerNoForwarder(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "ErrorCode", Value: "9002"},
	}
	return data, "The DNS server has no forwarders configured and recursion is disabled. Queries for non-authoritative zones will fail."
}
