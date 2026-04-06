package catalog

import (
	"fmt"
	"math/rand"
)

const (
	taskKerberos  = 14339
	taskCredValid = 14336
)

func init() {
	accountLogonEvents := []struct {
		id      int
		task    int
		kw      uint64
		minRole MachineRole
		gen     func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		// Kerberos events (DC only)
		{4768, taskKerberos, keywordsAuditSuccess, RoleDC, generateKerberosTGTRequested},
		{4769, taskKerberos, keywordsAuditSuccess, RoleDC, generateKerberosServiceTicket},
		{4770, taskKerberos, keywordsAuditSuccess, RoleDC, generateKerberosTicketRenewed},
		{4771, taskKerberos, keywordsAuditFailure, RoleDC, generateKerberosPreAuthFailed},
		// NTLM credential validation (all roles)
		{4776, taskCredValid, keywordsAuditSuccess, RoleWorkstation, generateNTLMCredentialValidation},
	}

	for _, ev := range accountLogonEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         ev.task,
			TaskName:     "Account Logon",
			Keywords:     ev.kw,
			KeywordNames: keywordNames(ev.kw),
			MinRole:      ev.minRole,
			Generate:     ev.gen,
		})
	}
}

func generateKerberosTGTRequested(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	ip := PickIP(r, opts.IPs)
	data := []EventDataField{
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "ServiceName", Value: "krbtgt"},
		{Name: "ServiceSid", Value: "S-1-5-21-0-0-0-502"},
		{Name: "TicketOptions", Value: RandomTicketOptions(r)},
		{Name: "Status", Value: "0x0"},
		{Name: "TicketEncryptionType", Value: RandomTicketEncryptionType(r)},
		{Name: "PreAuthType", Value: "15"},
		{Name: "IpAddress", Value: fmt.Sprintf("::ffff:%s", ip)},
		{Name: "IpPort", Value: RandomPort(r)},
		{Name: "CertIssuerName", Value: ""},
		{Name: "CertSerialNumber", Value: ""},
		{Name: "CertThumbprint", Value: ""},
	}
	msg := fmt.Sprintf("A Kerberos authentication ticket (TGT) was requested.\n\nAccount Information:\n\tAccount Name:\t\t%s\n\tSupplied Realm Name:\t%s",
		user, opts.DomainName)
	return data, msg
}

func generateKerberosServiceTicket(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	services := []string{
		"cifs/" + PickHostname(r, opts.Hostnames),
		"ldap/" + PickHostname(r, opts.Hostnames),
		"host/" + PickHostname(r, opts.Hostnames),
		"HTTP/" + PickHostname(r, opts.Hostnames),
		"MSSQLSvc/" + PickHostname(r, opts.Hostnames) + ":1433",
	}
	svc := services[r.Intn(len(services))] // #nosec G404
	data := []EventDataField{
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "ServiceName", Value: svc},
		{Name: "ServiceSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "TicketOptions", Value: RandomTicketOptions(r)},
		{Name: "TicketEncryptionType", Value: RandomTicketEncryptionType(r)},
		{Name: "IpAddress", Value: fmt.Sprintf("::ffff:%s", PickIP(r, opts.IPs))},
		{Name: "IpPort", Value: RandomPort(r)},
		{Name: "Status", Value: "0x0"},
		{Name: "LogonGuid", Value: RandomGUID(r)},
		{Name: "TransmittedServices", Value: "-"},
	}
	msg := fmt.Sprintf("A Kerberos service ticket was requested.\n\nAccount:\n\tAccount Name:\t\t%s\n\nService:\n\tService Name:\t\t%s",
		user, svc)
	return data, msg
}

func generateKerberosTicketRenewed(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TargetUserName", Value: user + "@" + opts.DomainName},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "ServiceName", Value: "krbtgt"},
		{Name: "ServiceSid", Value: "S-1-5-21-0-0-0-502"},
		{Name: "TicketOptions", Value: RandomTicketOptions(r)},
		{Name: "TicketEncryptionType", Value: RandomTicketEncryptionType(r)},
		{Name: "IpAddress", Value: fmt.Sprintf("::ffff:%s", PickIP(r, opts.IPs))},
		{Name: "IpPort", Value: RandomPort(r)},
	}
	msg := fmt.Sprintf("A Kerberos service ticket was renewed.\n\nAccount:\n\tAccount Name:\t\t%s",
		user)
	return data, msg
}

func generateKerberosPreAuthFailed(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	status := RandomKerberosStatus(r)
	data := []EventDataField{
		{Name: "TargetUserName", Value: user},
		{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "ServiceName", Value: "krbtgt/" + opts.DomainName},
		{Name: "TicketOptions", Value: RandomTicketOptions(r)},
		{Name: "Status", Value: status},
		{Name: "PreAuthType", Value: "15"},
		{Name: "IpAddress", Value: fmt.Sprintf("::ffff:%s", PickIP(r, opts.IPs))},
		{Name: "IpPort", Value: RandomPort(r)},
		{Name: "CertIssuerName", Value: ""},
		{Name: "CertSerialNumber", Value: ""},
		{Name: "CertThumbprint", Value: ""},
	}
	msg := fmt.Sprintf("Kerberos pre-authentication failed.\n\nAccount:\n\tAccount Name:\t\t%s\n\nFailure Code:\t%s",
		user, status)
	return data, msg
}

func generateNTLMCredentialValidation(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	host := PickHostname(r, opts.Hostnames)
	statuses := []string{"0x0", "0xC000006D", "0xC000006A"}
	status := statuses[r.Intn(len(statuses))] // #nosec G404
	data := []EventDataField{
		{Name: "LogonAccount", Value: user},
		{Name: "SourceWorkstation", Value: host},
		{Name: "Error", Value: status},
		{Name: "Workstation", Value: host},
	}
	msg := fmt.Sprintf("The computer attempted to validate the credentials for an account.\n\nLogon Account:\t%s\nSource Workstation:\t%s\nError Code:\t%s",
		user, host, status)
	return data, msg
}
