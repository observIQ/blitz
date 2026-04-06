package catalog

import (
	"fmt"
	"math/rand"
)

const (
	taskDSAccess  = 14080
	taskDSChanges = 14081
)

var adObjectDNs = []string{
	"CN=John Smith,OU=Users,DC=contoso,DC=com",
	"CN=Server01,OU=Servers,DC=contoso,DC=com",
	"CN=Domain Admins,CN=Users,DC=contoso,DC=com",
	"CN=Default Domain Policy,CN=System,DC=contoso,DC=com",
	"CN=NTDS Settings,CN=DC01,CN=Servers,CN=Default-First-Site-Name,CN=Sites,CN=Configuration,DC=contoso,DC=com",
}

var adObjectClasses = []string{
	"user", "computer", "group", "organizationalUnit",
	"groupPolicyContainer", "nTDSDSA", "domainDNS",
}

var adAttributeNames = []string{
	"member", "userAccountControl", "description", "displayName",
	"servicePrincipalName", "dNSHostName", "whenChanged",
	"msDS-AllowedToDelegateTo", "pwdLastSet", "lockoutTime",
}

func init() {
	dsEvents := []struct {
		id   int
		task int
		gen  func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4662, taskDSAccess, generateDSObjectOperation},
		{5136, taskDSChanges, generateDSObjectModified},
		{5137, taskDSChanges, generateDSObjectCreated},
		{5139, taskDSChanges, generateDSObjectMoved},
		{5141, taskDSChanges, generateDSObjectDeleted},
	}

	for _, ev := range dsEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         ev.task,
			TaskName:     "Directory Service Access",
			Keywords:     keywordsAuditSuccess,
			KeywordNames: []string{"Audit Success"},
			MinRole:      RoleDC,
			Generate:     ev.gen,
		})
	}
}

func generateDSObjectOperation(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	objDN := adObjectDNs[r.Intn(len(adObjectDNs))]            // #nosec G404
	objClass := adObjectClasses[r.Intn(len(adObjectClasses))] // #nosec G404
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectServer", Value: "DS"},
		EventDataField{Name: "ObjectType", Value: objClass},
		EventDataField{Name: "ObjectName", Value: objDN},
		EventDataField{Name: "HandleId", Value: "0x0"},
		EventDataField{Name: "AccessList", Value: "%%7688"},
		EventDataField{Name: "AccessMask", Value: RandomAccessMask(r)},
		EventDataField{Name: "Properties", Value: fmt.Sprintf("%%7688\n\t%s", objClass)},
		EventDataField{Name: "AdditionalInfo", Value: "-"},
		EventDataField{Name: "AdditionalInfo2", Value: "-"},
	)
	msg := fmt.Sprintf("An operation was performed on an object.\n\nSubject:\n\tAccount Name:\t\t%s\n\nDirectory Service:\n\tObject DN:\t%s",
		data[1].Value, objDN)
	return data, msg
}

func generateDSObjectModified(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	objDN := adObjectDNs[r.Intn(len(adObjectDNs))]            // #nosec G404
	objClass := adObjectClasses[r.Intn(len(adObjectClasses))] // #nosec G404
	attr := adAttributeNames[r.Intn(len(adAttributeNames))]   // #nosec G404
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectDN", Value: objDN},
		EventDataField{Name: "ObjectGUID", Value: RandomGUID(r)},
		EventDataField{Name: "ObjectClass", Value: objClass},
		EventDataField{Name: "AttributeLDAPDisplayName", Value: attr},
		EventDataField{Name: "AttributeSyntaxOID", Value: "2.5.5.1"},
		EventDataField{Name: "AttributeValue", Value: "updated-value"},
		EventDataField{Name: "OperationType", Value: "%%14674"},
		EventDataField{Name: "DSName", Value: opts.DomainName},
		EventDataField{Name: "DSType", Value: "%%14676"},
	)
	msg := fmt.Sprintf("A directory service object was modified.\n\nObject:\n\tDN:\t%s\n\tClass:\t%s\n\tAttribute:\t%s",
		objDN, objClass, attr)
	return data, msg
}

func generateDSObjectCreated(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	objDN := adObjectDNs[r.Intn(len(adObjectDNs))]            // #nosec G404
	objClass := adObjectClasses[r.Intn(len(adObjectClasses))] // #nosec G404
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectDN", Value: objDN},
		EventDataField{Name: "ObjectGUID", Value: RandomGUID(r)},
		EventDataField{Name: "ObjectClass", Value: objClass},
		EventDataField{Name: "DSName", Value: opts.DomainName},
		EventDataField{Name: "DSType", Value: "%%14676"},
	)
	msg := fmt.Sprintf("A directory service object was created.\n\nObject:\n\tDN:\t%s\n\tClass:\t%s",
		objDN, objClass)
	return data, msg
}

func generateDSObjectMoved(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	objDN := adObjectDNs[r.Intn(len(adObjectDNs))]            // #nosec G404
	objClass := adObjectClasses[r.Intn(len(adObjectClasses))] // #nosec G404
	newDN := "CN=Moved," + objDN[len("CN=x,"):]
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectDN", Value: objDN},
		EventDataField{Name: "ObjectGUID", Value: RandomGUID(r)},
		EventDataField{Name: "ObjectClass", Value: objClass},
		EventDataField{Name: "NewObjectDN", Value: newDN},
		EventDataField{Name: "DSName", Value: opts.DomainName},
		EventDataField{Name: "DSType", Value: "%%14676"},
	)
	msg := fmt.Sprintf("A directory service object was moved.\n\nOld DN:\t%s\nNew DN:\t%s", objDN, newDN)
	return data, msg
}

func generateDSObjectDeleted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	objDN := adObjectDNs[r.Intn(len(adObjectDNs))]            // #nosec G404
	objClass := adObjectClasses[r.Intn(len(adObjectClasses))] // #nosec G404
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectDN", Value: objDN},
		EventDataField{Name: "ObjectGUID", Value: RandomGUID(r)},
		EventDataField{Name: "ObjectClass", Value: objClass},
		EventDataField{Name: "DSName", Value: opts.DomainName},
		EventDataField{Name: "DSType", Value: "%%14676"},
	)
	msg := fmt.Sprintf("A directory service object was deleted.\n\nObject:\n\tDN:\t%s\n\tClass:\t%s",
		objDN, objClass)
	return data, msg
}
