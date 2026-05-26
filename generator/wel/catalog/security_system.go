package catalog

import (
	"fmt"
	"math/rand"
)

const (
	taskSecurityStateChange   = 12288
	taskSecuritySystemExt     = 12289
	taskSystemIntegrity       = 12290
	taskSecuritySystemOther   = 12292
	taskFilterPlatformConnect = 12293
)

func init() {
	systemEvents := []struct {
		id   int
		task int
		kw   uint64
		gen  func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{1102, taskSecuritySystemOther, keywordsAuditSuccess, generateAuditLogCleared},
		{4608, taskSecurityStateChange, keywordsAuditSuccess, generateWindowsStarting},
		{4610, taskSecuritySystemExt, keywordsAuditSuccess, generateAuthPackageLoaded},
		{4611, taskSecuritySystemExt, keywordsAuditSuccess, generateTrustedLogonProcess},
		{4616, taskSecurityStateChange, keywordsAuditSuccess, generateSystemTimeChanged},
		{4622, taskSecuritySystemExt, keywordsAuditSuccess, generateSecurityPackageLoaded},
		{5024, taskFilterPlatformConnect, keywordsAuditSuccess, generateFirewallStarted},
		{5025, taskFilterPlatformConnect, keywordsAuditSuccess, generateFirewallStopped},
		{5156, taskFilterPlatformConnect, keywordsAuditSuccess, generateWFPConnectionAllowed},
		{5157, taskFilterPlatformConnect, keywordsAuditFailure, generateWFPConnectionBlocked},
	}

	for _, ev := range systemEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         ev.task,
			TaskName:     "System Events",
			Keywords:     ev.kw,
			KeywordNames: keywordNames(ev.kw),
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateAuditLogCleared(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	data := subjectFields(r, opts)
	msg := fmt.Sprintf("The audit log was cleared.\n\nSubject:\n\tAccount Name:\t\t%s", data[1].Value)
	return data, msg
}

func generateWindowsStarting(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	return nil, "Windows is starting up."
}

func generateAuthPackageLoaded(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	packages := []string{"NTLM", "Kerberos", "Negotiate", "Schannel", "WDigest", "Microsoft_Authentication_Package_V1_0"}
	data := []EventDataField{
		{Name: "AuthenticationPackageName", Value: packages[0]},
	}
	msg := fmt.Sprintf("An authentication package has been loaded by the Local Security Authority.\n\nAuthentication Package Name:\t%s", packages[0])
	return data, msg
}

func generateTrustedLogonProcess(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	processes := []string{"User32", "Winlogon", "Advapi", "SCM"}
	proc := processes[r.Intn(len(processes))] // #nosec G404
	data := []EventDataField{
		{Name: "LogonProcessName", Value: proc},
	}
	msg := fmt.Sprintf("A trusted logon process has been registered with the Local Security Authority.\n\nLogon Process Name:\t%s", proc)
	return data, msg
}

func generateSystemTimeChanged(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	data := append(subjectFields(r, opts),
		EventDataField{Name: "PreviousTime", Value: "2024-03-15T10:30:00.0000000Z"},
		EventDataField{Name: "NewTime", Value: "2024-03-15T10:30:01.5000000Z"},
		EventDataField{Name: "ProcessId", Value: RandomProcessID(r)},
		EventDataField{Name: "ProcessName", Value: `C:\Windows\System32\svchost.exe`},
	)
	msg := fmt.Sprintf("The system time was changed.\n\nSubject:\n\tAccount Name:\t\t%s", data[1].Value)
	return data, msg
}

func generateSecurityPackageLoaded(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	data := []EventDataField{
		{Name: "SecurityPackageName", Value: "Schannel"},
	}
	msg := "A security package has been loaded by the Local Security Authority.\n\nSecurity Package Name:\tSchannel"
	return data, msg
}

func generateFirewallStarted(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	return nil, "The Windows Firewall Service has started successfully."
}

func generateFirewallStopped(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	return nil, "The Windows Firewall Service has been stopped."
}

func generateWFPConnectionAllowed(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	protocols := []string{"6", "17"}           // TCP, UDP
	proto := protocols[r.Intn(len(protocols))] // #nosec G404
	srcIP := PickIP(r, opts.IPs)
	dstIP := RandomIPv4(r)
	data := []EventDataField{
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Application", Value: `\device\harddiskvolume2\windows\system32\svchost.exe`},
		{Name: "Direction", Value: "%%14592"},
		{Name: "SourceAddress", Value: srcIP},
		{Name: "SourcePort", Value: RandomPort(r)},
		{Name: "DestAddress", Value: dstIP},
		{Name: "DestPort", Value: RandomPort(r)},
		{Name: "Protocol", Value: proto},
		{Name: "FilterRTID", Value: fmt.Sprintf("%d", r.Intn(100000)+10000)}, // #nosec G404
		{Name: "LayerName", Value: "%%14608"},
		{Name: "LayerRTID", Value: "44"},
	}
	msg := fmt.Sprintf("The Windows Filtering Platform has permitted a connection.\n\nSource Address:\t%s\nDestination Address:\t%s",
		srcIP, dstIP)
	return data, msg
}

func generateWFPConnectionBlocked(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	srcIP := RandomIPv4(r)
	dstIP := PickIP(r, opts.IPs)
	data := []EventDataField{
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Application", Value: `\device\harddiskvolume2\program files\unknown\app.exe`},
		{Name: "Direction", Value: "%%14592"},
		{Name: "SourceAddress", Value: srcIP},
		{Name: "SourcePort", Value: RandomPort(r)},
		{Name: "DestAddress", Value: dstIP},
		{Name: "DestPort", Value: RandomPort(r)},
		{Name: "Protocol", Value: "6"},
		{Name: "FilterRTID", Value: fmt.Sprintf("%d", r.Intn(100000)+10000)}, // #nosec G404
		{Name: "LayerName", Value: "%%14608"},
		{Name: "LayerRTID", Value: "44"},
	}
	msg := fmt.Sprintf("The Windows Filtering Platform has blocked a connection.\n\nSource Address:\t%s\nDestination Address:\t%s",
		srcIP, dstIP)
	return data, msg
}
