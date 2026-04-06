package catalog

import (
	"fmt"
	"math/rand"
)

const (
	taskAuditPolicyChange  = 13568
	taskAuthPolicyChange   = 13569
	taskMPSSvcRuleChange   = 13571
	taskFilterPolicyChange = 13570
)

func init() {
	policyEvents := []struct {
		id   int
		task int
		kw   uint64
		gen  func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4703, taskAuditPolicyChange, keywordsAuditSuccess, generateTokenRightAdjusted},
		{4704, taskAuditPolicyChange, keywordsAuditSuccess, generateUserRightAssigned},
		{4705, taskAuditPolicyChange, keywordsAuditSuccess, generateUserRightRemoved},
		{4719, taskAuditPolicyChange, keywordsAuditSuccess, generateAuditPolicyChanged},
		{4739, taskAuthPolicyChange, keywordsAuditSuccess, generateDomainPolicyChanged},
		{4946, taskMPSSvcRuleChange, keywordsAuditSuccess, generateFirewallRuleAdded},
		{4947, taskMPSSvcRuleChange, keywordsAuditSuccess, generateFirewallRuleModified},
		{4948, taskMPSSvcRuleChange, keywordsAuditSuccess, generateFirewallRuleDeleted},
	}

	for _, ev := range policyEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         ev.task,
			TaskName:     "Policy Change",
			Keywords:     ev.kw,
			KeywordNames: keywordNames(ev.kw),
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateTokenRightAdjusted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	enabledPrivs := RandomPrivilegeList(r)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "EnabledPrivilegeList", Value: enabledPrivs},
		EventDataField{Name: "DisabledPrivilegeList", Value: "-"},
		EventDataField{Name: "ProcessId", Value: RandomProcessID(r)},
		EventDataField{Name: "ProcessName", Value: `C:\Windows\System32\svchost.exe`},
	)
	msg := fmt.Sprintf("A token right was adjusted.\n\nSubject:\n\tAccount Name:\t\t%s\n\nProcess:\n\tProcess Name:\tC:\\Windows\\System32\\svchost.exe",
		user)
	return data, msg
}

func generateUserRightAssigned(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	priv := Privileges[r.Intn(len(Privileges))] // #nosec G404
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "PrivilegeList", Value: priv},
	)
	msg := fmt.Sprintf("A user right was assigned.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s\n\nNew Right:\t%s",
		data[1].Value, target, priv)
	return data, msg
}

func generateUserRightRemoved(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	priv := Privileges[r.Intn(len(Privileges))] // #nosec G404
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "PrivilegeList", Value: priv},
	)
	msg := fmt.Sprintf("A user right was removed.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s\n\nRemoved Right:\t%s",
		data[1].Value, target, priv)
	return data, msg
}

func generateAuditPolicyChanged(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	categories := []string{
		"Account Logon", "Account Management", "Detailed Tracking",
		"DS Access", "Logon/Logoff", "Object Access",
		"Policy Change", "Privilege Use", "System",
	}
	cat := categories[r.Intn(len(categories))] // #nosec G404
	subcats := []string{"Credential Validation", "Kerberos Authentication Service", "Logon", "Logoff", "Process Creation"}
	subcat := subcats[r.Intn(len(subcats))] // #nosec G404

	data := append(subjectFields(r, opts),
		EventDataField{Name: "CategoryId", Value: cat},
		EventDataField{Name: "SubcategoryId", Value: subcat},
		EventDataField{Name: "SubcategoryGuid", Value: RandomGUID(r)},
		EventDataField{Name: "AuditPolicyChanges", Value: "%%8448"},
	)
	msg := fmt.Sprintf("System audit policy was changed.\n\nSubject:\n\tAccount Name:\t\t%s\n\nAudit Policy Change:\n\tCategory:\t\t%s\n\tSubcategory:\t\t%s",
		data[1].Value, cat, subcat)
	return data, msg
}

func generateDomainPolicyChanged(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	data := append(subjectFields(r, opts),
		EventDataField{Name: "DomainName", Value: opts.DomainName},
		EventDataField{Name: "DomainSid", Value: "S-1-5-21-0-0-0"},
		EventDataField{Name: "ForceLogoff", Value: "%%1794"},
		EventDataField{Name: "LockoutThreshold", Value: "5"},
		EventDataField{Name: "LockoutObservationWindow", Value: "1800"},
		EventDataField{Name: "LockoutDuration", Value: "1800"},
		EventDataField{Name: "PasswordProperties", Value: "1"},
		EventDataField{Name: "MinPasswordAge", Value: "86400"},
		EventDataField{Name: "MaxPasswordAge", Value: "7776000"},
		EventDataField{Name: "MinPasswordLength", Value: "8"},
		EventDataField{Name: "PasswordHistoryLength", Value: "24"},
		EventDataField{Name: "MachineAccountQuota", Value: "10"},
		EventDataField{Name: "MixedDomainMode", Value: "%%1794"},
		EventDataField{Name: "DomainBehaviorVersion", Value: "7"},
		EventDataField{Name: "OemInformation", Value: "-"},
	)
	msg := fmt.Sprintf("Domain Policy was changed.\n\nSubject:\n\tAccount Name:\t\t%s\n\nDomain:\t%s", data[1].Value, opts.DomainName)
	return data, msg
}

func generateFirewallRuleAdded(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	ruleName := fmt.Sprintf("Allow-%s-In", []string{"HTTP", "HTTPS", "RDP", "SMB", "DNS", "WinRM"}[r.Intn(6)]) // #nosec G404
	data := []EventDataField{
		{Name: "ProfileChanged", Value: "All"},
		{Name: "RuleId", Value: RandomGUID(r)},
		{Name: "RuleName", Value: ruleName},
	}
	msg := fmt.Sprintf("A rule was added to the Windows Firewall exception list.\n\nRule Name:\t%s", ruleName)
	return data, msg
}

func generateFirewallRuleModified(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	ruleName := fmt.Sprintf("Allow-%s-In", []string{"HTTP", "HTTPS", "RDP", "SMB"}[r.Intn(4)]) // #nosec G404
	data := []EventDataField{
		{Name: "ProfileChanged", Value: "Domain"},
		{Name: "RuleId", Value: RandomGUID(r)},
		{Name: "RuleName", Value: ruleName},
	}
	msg := fmt.Sprintf("A rule was modified in the Windows Firewall exception list.\n\nRule Name:\t%s", ruleName)
	return data, msg
}

func generateFirewallRuleDeleted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	ruleName := fmt.Sprintf("Allow-%s-In", []string{"HTTP", "HTTPS", "RDP"}[r.Intn(3)]) // #nosec G404
	data := []EventDataField{
		{Name: "ProfileChanged", Value: "All"},
		{Name: "RuleId", Value: RandomGUID(r)},
		{Name: "RuleName", Value: ruleName},
	}
	msg := fmt.Sprintf("A rule was deleted from the Windows Firewall exception list.\n\nRule Name:\t%s", ruleName)
	return data, msg
}
