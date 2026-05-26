package catalog

import (
	"fmt"
	"math/rand"
)

const taskPrivilegeUse = 13056

func init() {
	for _, ev := range []struct {
		id  int
		gen func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4672, generateSpecialPrivilegesAssigned},
		{4673, generatePrivilegedServiceCalled},
		{4674, generatePrivilegedObjectOperation},
	} {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         taskPrivilegeUse,
			TaskName:     "Privilege Use",
			Keywords:     keywordsAuditSuccess,
			KeywordNames: []string{"Audit Success"},
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateSpecialPrivilegesAssigned(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "SubjectUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "SubjectUserName", Value: user},
		{Name: "SubjectDomainName", Value: opts.DomainName},
		{Name: "SubjectLogonId", Value: RandomLogonID(r)},
		{Name: "PrivilegeList", Value: RandomPrivilegeList(r)},
	}
	msg := fmt.Sprintf("Special privileges assigned to new logon.\n\nSubject:\n\tAccount Name:\t\t%s\n\nPrivileges:\t\t%s",
		user, data[4].Value)
	return data, msg
}

func generatePrivilegedServiceCalled(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "SubjectUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "SubjectUserName", Value: user},
		{Name: "SubjectDomainName", Value: opts.DomainName},
		{Name: "SubjectLogonId", Value: RandomLogonID(r)},
		{Name: "ObjectServer", Value: "Security"},
		{Name: "Service", Value: "LsaRegisterLogonProcess()"},
		{Name: "PrivilegeList", Value: RandomPrivilegeList(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "ProcessName", Value: `C:\Windows\System32\lsass.exe`},
	}
	msg := fmt.Sprintf("A privileged service was called.\n\nSubject:\n\tAccount Name:\t\t%s", user)
	return data, msg
}

func generatePrivilegedObjectOperation(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "SubjectUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "SubjectUserName", Value: user},
		{Name: "SubjectDomainName", Value: opts.DomainName},
		{Name: "SubjectLogonId", Value: RandomLogonID(r)},
		{Name: "ObjectServer", Value: "Security"},
		{Name: "ObjectType", Value: "File"},
		{Name: "ObjectName", Value: `C:\Windows\System32\config\SAM`},
		{Name: "HandleId", Value: RandomHexID(r, 4)},
		{Name: "AccessList", Value: "%%4416"},
		{Name: "AccessMask", Value: RandomAccessMask(r)},
		{Name: "PrivilegeList", Value: RandomPrivilegeList(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "ProcessName", Value: `C:\Windows\System32\lsass.exe`},
	}
	msg := fmt.Sprintf("An operation was attempted on a privileged object.\n\nSubject:\n\tAccount Name:\t\t%s\n\nObject:\n\tObject Name:\t\t%s",
		user, data[6].Value)
	return data, msg
}
