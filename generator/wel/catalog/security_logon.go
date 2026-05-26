package catalog

import (
	"fmt"
	"math/rand"
)

const (
	securityProvider     = "Microsoft-Windows-Security-Auditing"
	securityProviderGUID = "{54849625-5478-4994-a5ba-3e3b0328c30d}"

	keywordsAuditSuccess uint64 = 0x8020000000000000
	keywordsAuditFailure uint64 = 0x8010000000000000

	taskLogon  = 12544
	taskLogoff = 12545
)

func init() {
	// 4624 - Successful logon
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4624,
		Version:      2,
		Level:        LevelLogAlways,
		Task:         taskLogon,
		TaskName:     "Logon",
		Opcode:       0,
		OpcodeName:   "Info",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateLogonSuccess,
	})

	// 4625 - Failed logon
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4625,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogon,
		TaskName:     "Logon",
		Keywords:     keywordsAuditFailure,
		KeywordNames: []string{"Audit Failure"},
		MinRole:      RoleWorkstation,
		Generate:     generateLogonFailure,
	})

	// 4634 - Logoff
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4634,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogoff,
		TaskName:     "Logoff",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateLogoff,
	})

	// 4647 - User initiated logoff
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4647,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogoff,
		TaskName:     "Logoff",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateUserLogoff,
	})

	// 4648 - Logon using explicit credentials
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4648,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogon,
		TaskName:     "Logon",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateExplicitCredLogon,
	})

	// 4778 - Session reconnected
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4778,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogon,
		TaskName:     "Logon",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateSessionReconnected,
	})

	// 4779 - Session disconnected
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4779,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogoff,
		TaskName:     "Logoff",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateSessionDisconnected,
	})

	// 4800 - Workstation locked
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4800,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogon,
		TaskName:     "Logon",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateWorkstationLocked,
	})

	// 4801 - Workstation unlocked
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4801,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogon,
		TaskName:     "Logon",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateWorkstationUnlocked,
	})

	// 4802 - Screen saver invoked
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4802,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogon,
		TaskName:     "Logon",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateScreenSaverInvoked,
	})

	// 4803 - Screen saver dismissed
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4803,
		Version:      0,
		Level:        LevelLogAlways,
		Task:         taskLogon,
		TaskName:     "Logon",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateScreenSaverDismissed,
	})
}

func generateLogonSuccess(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	logonID := RandomLogonID(r)
	logonType := RandomLogonType(r)
	ip := PickIP(r, opts.IPs)

	// Track the logon session
	if opts.State != nil {
		opts.State.AddLogonSession(logonID, user, opts.DomainName)
	}

	data := []EventDataField{
		{Name: "SubjectUserSid", Value: "S-1-5-18"},
		{Name: "SubjectUserName", Value: opts.Computer + "$"},
		{Name: "SubjectDomainName", Value: opts.DomainName},
		{Name: "SubjectLogonId", Value: "0x3e7"},
		{Name: "TargetUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetLogonId", Value: logonID},
		{Name: "LogonType", Value: logonType},
		{Name: "LogonProcessName", Value: RandomLogonProcess(r)},
		{Name: "AuthenticationPackageName", Value: RandomAuthPackage(r)},
		{Name: "WorkstationName", Value: PickHostname(r, opts.Hostnames)},
		{Name: "LogonGuid", Value: RandomGUID(r)},
		{Name: "TransmittedServices", Value: "-"},
		{Name: "LmPackageName", Value: "-"},
		{Name: "KeyLength", Value: "128"},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "ProcessName", Value: `C:\Windows\System32\svchost.exe`},
		{Name: "IpAddress", Value: ip},
		{Name: "IpPort", Value: RandomPort(r)},
		{Name: "ImpersonationLevel", Value: RandomImpersonationLevel(r)},
		{Name: "RestrictedAdminMode", Value: "-"},
		{Name: "VirtualAccount", Value: "%%1843"},
		{Name: "ElevatedToken", Value: RandomElevationType(r)},
	}

	msg := fmt.Sprintf("An account was successfully logged on.\n\nSubject:\n\tSecurity ID:\t\tS-1-5-18\n\tAccount Name:\t\t%s$\n\tLogon Type:\t\t%s\n\nNew Logon:\n\tSecurity ID:\t\t%s\n\tAccount Name:\t\t%s",
		opts.Computer, logonType, data[4].Value, user)

	return data, msg
}

func generateLogonFailure(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	ip := PickIP(r, opts.IPs)
	logonType := RandomLogonType(r)

	// Status codes for common failure reasons
	statusCodes := []struct{ status, subStatus, reason string }{
		{"0xC000006D", "0xC000006A", "Unknown user name or bad password"},
		{"0xC000006D", "0xC0000064", "User logon with misspelled or bad user account"},
		{"0xC0000234", "0x0", "User logon with account locked"},
		{"0xC0000072", "0x0", "User logon to account disabled by administrator"},
		{"0xC000006F", "0x0", "User logon outside authorized hours"},
		{"0xC0000071", "0x0", "User logon with expired password"},
	}
	s := statusCodes[r.Intn(len(statusCodes))] // #nosec G404

	data := []EventDataField{
		{Name: "SubjectUserSid", Value: "S-1-0-0"},
		{Name: "SubjectUserName", Value: "-"},
		{Name: "SubjectDomainName", Value: "-"},
		{Name: "SubjectLogonId", Value: "0x0"},
		{Name: "TargetUserSid", Value: "S-1-0-0"},
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "Status", Value: s.status},
		{Name: "FailureReason", Value: s.reason},
		{Name: "SubStatus", Value: s.subStatus},
		{Name: "LogonType", Value: logonType},
		{Name: "LogonProcessName", Value: RandomLogonProcess(r)},
		{Name: "AuthenticationPackageName", Value: RandomAuthPackage(r)},
		{Name: "WorkstationName", Value: PickHostname(r, opts.Hostnames)},
		{Name: "TransmittedServices", Value: "-"},
		{Name: "LmPackageName", Value: "-"},
		{Name: "KeyLength", Value: "0"},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "ProcessName", Value: `C:\Windows\System32\svchost.exe`},
		{Name: "IpAddress", Value: ip},
		{Name: "IpPort", Value: RandomPort(r)},
	}

	msg := fmt.Sprintf("An account failed to log on.\n\nSubject:\n\tAccount Name:\t\t%s\n\tLogon Type:\t\t%s\n\tFailure Reason:\t\t%s",
		user, logonType, s.reason)

	return data, msg
}

func generateLogoff(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	logonID := RandomLogonID(r)
	logonType := RandomLogonType(r)

	// Try to pick from state tracker
	if opts.State != nil {
		if session, ok := opts.State.PickLogonSession(); ok {
			user = session.Username
			logonID = session.LogonID
			opts.State.RemoveLogonSession(logonID)
		}
	}

	data := []EventDataField{
		{Name: "TargetUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetLogonId", Value: logonID},
		{Name: "LogonType", Value: logonType},
	}

	msg := fmt.Sprintf("An account was logged off.\n\nSubject:\n\tAccount Name:\t\t%s\n\tLogon ID:\t\t%s",
		user, logonID)

	return data, msg
}

func generateUserLogoff(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	logonID := RandomLogonID(r)

	if opts.State != nil {
		if session, ok := opts.State.PickLogonSession(); ok {
			user = session.Username
			logonID = session.LogonID
			opts.State.RemoveLogonSession(logonID)
		}
	}

	data := []EventDataField{
		{Name: "TargetUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetLogonId", Value: logonID},
	}

	msg := fmt.Sprintf("User initiated logoff:\n\nSubject:\n\tAccount Name:\t\t%s\n\tLogon ID:\t\t%s",
		user, logonID)

	return data, msg
}

func generateExplicitCredLogon(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subjectUser := PickUsername(r, opts.Usernames)
	targetUser := PickUsername(r, opts.Usernames)
	ip := PickIP(r, opts.IPs)

	data := []EventDataField{
		{Name: "SubjectUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "SubjectUserName", Value: subjectUser},
		{Name: "SubjectDomainName", Value: opts.DomainName},
		{Name: "SubjectLogonId", Value: RandomLogonID(r)},
		{Name: "LogonGuid", Value: RandomGUID(r)},
		{Name: "TargetUserName", Value: targetUser},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetLogonGuid", Value: RandomGUID(r)},
		{Name: "TargetServerName", Value: PickHostname(r, opts.Hostnames)},
		{Name: "TargetInfo", Value: PickHostname(r, opts.Hostnames)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "ProcessName", Value: `C:\Windows\System32\svchost.exe`},
		{Name: "IpAddress", Value: ip},
		{Name: "IpPort", Value: RandomPort(r)},
	}

	msg := fmt.Sprintf("A logon was attempted using explicit credentials.\n\nSubject:\n\tAccount Name:\t\t%s\n\nAccount Whose Credentials Were Used:\n\tAccount Name:\t\t%s",
		subjectUser, targetUser)

	return data, msg
}

func generateSessionReconnected(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "AccountName", Value: user},
		{Name: "AccountDomain", Value: opts.DomainName},
		{Name: "LogonID", Value: RandomLogonID(r)},
		{Name: "SessionName", Value: fmt.Sprintf("RDP-Tcp#%d", r.Intn(100))}, // #nosec G404
		{Name: "ClientName", Value: PickHostname(r, opts.Hostnames)},
		{Name: "ClientAddress", Value: PickIP(r, opts.IPs)},
	}
	msg := fmt.Sprintf("A session was reconnected to a Window Station.\n\nAccount Name:\t%s\nSession Name:\t%s",
		user, data[3].Value)
	return data, msg
}

func generateSessionDisconnected(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "AccountName", Value: user},
		{Name: "AccountDomain", Value: opts.DomainName},
		{Name: "LogonID", Value: RandomLogonID(r)},
		{Name: "SessionName", Value: fmt.Sprintf("RDP-Tcp#%d", r.Intn(100))}, // #nosec G404
		{Name: "ClientName", Value: PickHostname(r, opts.Hostnames)},
		{Name: "ClientAddress", Value: PickIP(r, opts.IPs)},
	}
	msg := fmt.Sprintf("A session was disconnected from a Window Station.\n\nAccount Name:\t%s\nSession Name:\t%s",
		user, data[3].Value)
	return data, msg
}

func generateWorkstationLocked(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TargetUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetLogonId", Value: RandomLogonID(r)},
		{Name: "SessionId", Value: fmt.Sprintf("%d", r.Intn(10)+1)}, // #nosec G404
	}
	msg := fmt.Sprintf("The workstation was locked.\n\nSubject:\n\tAccount Name:\t\t%s", user)
	return data, msg
}

func generateWorkstationUnlocked(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TargetUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetLogonId", Value: RandomLogonID(r)},
		{Name: "SessionId", Value: fmt.Sprintf("%d", r.Intn(10)+1)}, // #nosec G404
	}
	msg := fmt.Sprintf("The workstation was unlocked.\n\nSubject:\n\tAccount Name:\t\t%s", user)
	return data, msg
}

func generateScreenSaverInvoked(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TargetUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetLogonId", Value: RandomLogonID(r)},
		{Name: "SessionId", Value: fmt.Sprintf("%d", r.Intn(10)+1)}, // #nosec G404
	}
	msg := fmt.Sprintf("The screen saver was invoked.\n\nSubject:\n\tAccount Name:\t\t%s", user)
	return data, msg
}

func generateScreenSaverDismissed(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TargetUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "TargetUserName", Value: user},
		{Name: "TargetDomainName", Value: opts.DomainName},
		{Name: "TargetLogonId", Value: RandomLogonID(r)},
		{Name: "SessionId", Value: fmt.Sprintf("%d", r.Intn(10)+1)}, // #nosec G404
	}
	msg := fmt.Sprintf("The screen saver was dismissed.\n\nSubject:\n\tAccount Name:\t\t%s", user)
	return data, msg
}
