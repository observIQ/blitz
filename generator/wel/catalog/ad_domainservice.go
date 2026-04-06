package catalog

import (
	"fmt"
	"math/rand"
)

const adDomainServiceChannel = "Microsoft-Windows-ActiveDirectory_DomainService/Operational"

func init() {
	adProvider := "Microsoft-Windows-ActiveDirectory_DomainService"
	adGUID := "{0e8478c5-3605-4e8c-8497-1e3a77b52070}"

	adEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{1644, LevelInformation, generateADExpensiveSearch},
		{2886, LevelWarning, generateADInsecureLDAP},
		{2887, LevelInformation, generateADLDAPSigningStats},
		{3000, LevelInformation, generateADOnlineDefrag},
	}

	for _, ev := range adEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      adDomainServiceChannel,
			Provider:     adProvider,
			ProviderGUID: adGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleDC,
			Generate:     ev.gen,
		})
	}
}

func generateADExpensiveSearch(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	filters := []string{
		"(&(objectCategory=person)(objectClass=user))",
		"(&(objectClass=computer)(operatingSystem=Windows*))",
		"(memberOf=CN=Domain Admins,CN=Users,DC=contoso,DC=com)",
	}
	filter := filters[r.Intn(len(filters))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	ip := PickIP(r, opts.IPs)
	data := []EventDataField{
		{Name: "Client", Value: fmt.Sprintf("%s:%s", ip, RandomPort(r))},
		{Name: "StartingNode", Value: "DC=contoso,DC=com"},
		{Name: "Filter", Value: filter},
		{Name: "SearchScope", Value: "Subtree"},
		{Name: "VisitedEntries", Value: fmt.Sprintf("%d", r.Intn(50000)+10000)}, // #nosec G404
		{Name: "ReturnedEntries", Value: fmt.Sprintf("%d", r.Intn(1000)+100)},   // #nosec G404
		{Name: "UsedIndexes", Value: "idx_objectCategory"},
		{Name: "PagesReferenced", Value: fmt.Sprintf("%d", r.Intn(5000)+500)}, // #nosec G404
		{Name: "PagesReadFromDisk", Value: fmt.Sprintf("%d", r.Intn(100))},    // #nosec G404
		{Name: "PagesPreReadFromDisk", Value: "0"},
		{Name: "SortedPages", Value: "0"},
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("Internal event: An LDAP search with a potentially expensive filter was executed.\nFilter: %s\nUser: %s\\%s",
		filter, opts.DomainName, user)
}

func generateADInsecureLDAP(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "Detail", Value: "This directory server is currently not requiring clients to use signing for LDAP binds."},
	}
	return data, "The security of this directory server can be significantly enhanced by configuring the server to reject Simple Authentication and Security Layer (SASL) LDAP binds that do not request signing."
}

func generateADLDAPSigningStats(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "UnsignedBinds", Value: fmt.Sprintf("%d", r.Intn(100))},     // #nosec G404
		{Name: "SignedBinds", Value: fmt.Sprintf("%d", r.Intn(10000)+100)}, // #nosec G404
		{Name: "Period", Value: "24 hours"},
	}
	return data, fmt.Sprintf("During the previous 24 hour period, the number of unsigned LDAP binds was %s and the number of signed LDAP binds was %s.",
		data[0].Value, data[1].Value)
}

func generateADOnlineDefrag(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	freedMB := r.Intn(500) + 50 // #nosec G404
	data := []EventDataField{
		{Name: "FreedSpace", Value: fmt.Sprintf("%d MB", freedMB)},
		{Name: "Duration", Value: fmt.Sprintf("%d seconds", r.Intn(300)+60)}, // #nosec G404
	}
	return data, fmt.Sprintf("Online defragmentation has completed a full pass on directory database. %d megabytes of free disk space have been reclaimed.", freedMB)
}
