package catalog

import (
	"fmt"
	"math/rand"
)

const sysmonChannel = "Microsoft-Windows-Sysmon/Operational"

func init() {
	sysmonProvider := "Microsoft-Windows-Sysmon"
	sysmonGUID := "{5770385f-c22a-43e0-bf4c-06f5698ffbd9}"

	sysmonEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{1, LevelInformation, generateSysmonProcessCreate},
		{3, LevelInformation, generateSysmonNetworkConnect},
		{5, LevelInformation, generateSysmonProcessTerminate},
		{7, LevelInformation, generateSysmonImageLoad},
		{11, LevelInformation, generateSysmonFileCreate},
		{13, LevelInformation, generateSysmonRegistryValueSet},
		{22, LevelInformation, generateSysmonDNSQuery},
	}

	for _, ev := range sysmonEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      sysmonChannel,
			Provider:     sysmonProvider,
			ProviderGUID: sysmonGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateSysmonProcessCreate(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	cmdLine := commandLines[r.Intn(len(commandLines))] // #nosec G404
	processName := filePaths[r.Intn(len(filePaths))]   // #nosec G404
	data := []EventDataField{
		{Name: "RuleName", Value: "-"},
		{Name: "UtcTime", Value: "2024-03-15 10:30:00.000"},
		{Name: "ProcessGuid", Value: RandomGUID(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Image", Value: processName},
		{Name: "FileVersion", Value: "10.0.19041.1"},
		{Name: "Description", Value: "Host Process for Windows Services"},
		{Name: "Product", Value: "Microsoft Windows Operating System"},
		{Name: "Company", Value: "Microsoft Corporation"},
		{Name: "OriginalFileName", Value: "svchost.exe"},
		{Name: "CommandLine", Value: cmdLine},
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
		{Name: "LogonGuid", Value: RandomGUID(r)},
		{Name: "LogonId", Value: RandomLogonID(r)},
		{Name: "TerminalSessionId", Value: "0"},
		{Name: "IntegrityLevel", Value: "System"},
		{Name: "Hashes", Value: fmt.Sprintf("SHA256=%s", RandomHexID(r, 32))},
		{Name: "ParentProcessGuid", Value: RandomGUID(r)},
		{Name: "ParentProcessId", Value: RandomProcessID(r)},
		{Name: "ParentImage", Value: `C:\Windows\System32\services.exe`},
		{Name: "ParentCommandLine", Value: `C:\Windows\System32\services.exe`},
	}
	return data, fmt.Sprintf("Process Create:\nImage: %s\nCommandLine: %s\nUser: %s\\%s",
		processName, cmdLine, opts.DomainName, user)
}

func generateSysmonNetworkConnect(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	srcIP := PickIP(r, opts.IPs)
	dstIP := RandomIPv4(r)
	dstPort := RandomPort(r)
	data := []EventDataField{
		{Name: "RuleName", Value: "-"},
		{Name: "UtcTime", Value: "2024-03-15 10:30:00.000"},
		{Name: "ProcessGuid", Value: RandomGUID(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Image", Value: `C:\Windows\System32\svchost.exe`},
		{Name: "User", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, PickUsername(r, opts.Usernames))},
		{Name: "Protocol", Value: "tcp"},
		{Name: "Initiated", Value: "true"},
		{Name: "SourceIsIpv6", Value: "false"},
		{Name: "SourceIp", Value: srcIP},
		{Name: "SourceHostname", Value: opts.Computer},
		{Name: "SourcePort", Value: RandomPort(r)},
		{Name: "DestinationIsIpv6", Value: "false"},
		{Name: "DestinationIp", Value: dstIP},
		{Name: "DestinationHostname", Value: ""},
		{Name: "DestinationPort", Value: dstPort},
	}
	return data, fmt.Sprintf("Network connection detected:\nImage: C:\\Windows\\System32\\svchost.exe\nDestinationIp: %s\nDestinationPort: %s",
		dstIP, dstPort)
}

func generateSysmonProcessTerminate(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	processName := filePaths[r.Intn(len(filePaths))] // #nosec G404
	data := []EventDataField{
		{Name: "RuleName", Value: "-"},
		{Name: "UtcTime", Value: "2024-03-15 10:30:00.000"},
		{Name: "ProcessGuid", Value: RandomGUID(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Image", Value: processName},
	}
	_ = opts
	return data, fmt.Sprintf("Process terminated:\nImage: %s", processName)
}

func generateSysmonImageLoad(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	dlls := []string{
		`C:\Windows\System32\ntdll.dll`,
		`C:\Windows\System32\kernel32.dll`,
		`C:\Windows\System32\user32.dll`,
		`C:\Windows\System32\advapi32.dll`,
		`C:\Windows\System32\msvcrt.dll`,
	}
	dll := dlls[r.Intn(len(dlls))] // #nosec G404
	data := []EventDataField{
		{Name: "RuleName", Value: "-"},
		{Name: "UtcTime", Value: "2024-03-15 10:30:00.000"},
		{Name: "ProcessGuid", Value: RandomGUID(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Image", Value: `C:\Windows\System32\svchost.exe`},
		{Name: "ImageLoaded", Value: dll},
		{Name: "Signed", Value: "true"},
		{Name: "Signature", Value: "Microsoft Windows"},
		{Name: "SignatureStatus", Value: "Valid"},
		{Name: "Hashes", Value: fmt.Sprintf("SHA256=%s", RandomHexID(r, 32))},
	}
	_ = opts
	return data, fmt.Sprintf("Image loaded:\nImage: C:\\Windows\\System32\\svchost.exe\nImageLoaded: %s", dll)
}

func generateSysmonFileCreate(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	targets := []string{
		`C:\Users\Public\Downloads\document.pdf`,
		`C:\Windows\Temp\tmp1234.tmp`,
		`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Startup\updater.lnk`,
	}
	target := targets[r.Intn(len(targets))] // #nosec G404
	data := []EventDataField{
		{Name: "RuleName", Value: "-"},
		{Name: "UtcTime", Value: "2024-03-15 10:30:00.000"},
		{Name: "ProcessGuid", Value: RandomGUID(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Image", Value: `C:\Windows\explorer.exe`},
		{Name: "TargetFilename", Value: target},
		{Name: "CreationUtcTime", Value: "2024-03-15 10:30:00.000"},
	}
	_ = opts
	return data, fmt.Sprintf("File created:\nImage: C:\\Windows\\explorer.exe\nTargetFilename: %s", target)
}

func generateSysmonRegistryValueSet(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	regPaths := []string{
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run\Updater`,
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced\Hidden`,
		`HKLM\SYSTEM\CurrentControlSet\Services\TestService\Start`,
	}
	regPath := regPaths[r.Intn(len(regPaths))] // #nosec G404
	data := []EventDataField{
		{Name: "RuleName", Value: "-"},
		{Name: "EventType", Value: "SetValue"},
		{Name: "UtcTime", Value: "2024-03-15 10:30:00.000"},
		{Name: "ProcessGuid", Value: RandomGUID(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Image", Value: `C:\Windows\regedit.exe`},
		{Name: "TargetObject", Value: regPath},
		{Name: "Details", Value: "DWORD (0x00000001)"},
	}
	_ = opts
	return data, fmt.Sprintf("Registry value set:\nImage: C:\\Windows\\regedit.exe\nTargetObject: %s", regPath)
}

func generateSysmonDNSQuery(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	queries := []string{
		"www.contoso.com", "mail.contoso.com", "dc01.contoso.com",
		"update.microsoft.com", "login.microsoftonline.com",
		"suspicious-domain.example.com",
	}
	query := queries[r.Intn(len(queries))] // #nosec G404
	data := []EventDataField{
		{Name: "RuleName", Value: "-"},
		{Name: "UtcTime", Value: "2024-03-15 10:30:00.000"},
		{Name: "ProcessGuid", Value: RandomGUID(r)},
		{Name: "ProcessId", Value: RandomProcessID(r)},
		{Name: "Image", Value: `C:\Windows\System32\svchost.exe`},
		{Name: "QueryName", Value: query},
		{Name: "QueryType", Value: "A"},
		{Name: "QueryStatus", Value: "0"},
		{Name: "QueryResults", Value: RandomIPv4(r)},
	}
	_ = opts
	return data, fmt.Sprintf("DNS query:\nImage: C:\\Windows\\System32\\svchost.exe\nQueryName: %s", query)
}
