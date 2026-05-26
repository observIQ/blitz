package catalog

import (
	"fmt"
	"math/rand"
)

const (
	taskProcessCreation    = 13312
	taskProcessTermination = 13313
)

var commandLines = []string{
	`C:\Windows\System32\svchost.exe -k netsvcs -p`,
	`C:\Windows\System32\cmd.exe /c dir`,
	`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe -NoProfile -Command "Get-Process"`,
	`C:\Program Files\Company\app.exe --config config.yml`,
	`C:\Windows\System32\taskhostw.exe`,
	`C:\Windows\System32\conhost.exe 0x4`,
	`"C:\Program Files\Windows Defender\MsMpEng.exe"`,
	`C:\Windows\explorer.exe`,
	`C:\Windows\System32\wbem\WmiPrvSE.exe -secured -Embedding`,
}

func init() {
	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4688,
		Level:        LevelLogAlways,
		Task:         taskProcessCreation,
		TaskName:     "Process Creation",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateProcessCreated,
	})

	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4689,
		Level:        LevelLogAlways,
		Task:         taskProcessTermination,
		TaskName:     "Process Termination",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateProcessExited,
	})

	Register(EventDefinition{
		Channel:      "Security",
		Provider:     securityProvider,
		ProviderGUID: securityProviderGUID,
		EventID:      4697,
		Level:        LevelLogAlways,
		Task:         taskProcessCreation,
		TaskName:     "Process Creation",
		Keywords:     keywordsAuditSuccess,
		KeywordNames: []string{"Audit Success"},
		MinRole:      RoleWorkstation,
		Generate:     generateServiceInstalled,
	})
}

func generateProcessCreated(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	newPID := RandomProcessID(r)
	parentPID := RandomProcessID(r)
	cmdLine := commandLines[r.Intn(len(commandLines))] // #nosec G404
	processName := filePaths[r.Intn(len(filePaths))]   // #nosec G404

	// Track process
	if opts.State != nil {
		opts.State.AddProcess(newPID, processName, user)
	}

	data := append(subjectFields(r, opts),
		EventDataField{Name: "NewProcessId", Value: newPID},
		EventDataField{Name: "NewProcessName", Value: processName},
		EventDataField{Name: "TokenElevationType", Value: RandomElevationType(r)},
		EventDataField{Name: "ProcessId", Value: parentPID},
		EventDataField{Name: "CommandLine", Value: cmdLine},
		EventDataField{Name: "TargetUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "TargetUserName", Value: user},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetLogonId", Value: RandomLogonID(r)},
		EventDataField{Name: "ParentProcessName", Value: `C:\Windows\System32\svchost.exe`},
		EventDataField{Name: "MandatoryLabel", Value: RandomMandatoryLabel(r)},
	)

	msg := fmt.Sprintf("A new process has been created.\n\nCreator Subject:\n\tAccount Name:\t\t%s\n\nProcess Information:\n\tNew Process ID:\t\t%s\n\tNew Process Name:\t%s\n\tCommand Line:\t\t%s",
		data[1].Value, newPID, processName, cmdLine)
	return data, msg
}

func generateProcessExited(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	pid := RandomProcessID(r)
	processName := `C:\Windows\System32\svchost.exe`

	// Try to pick from tracked processes
	if opts.State != nil {
		if proc, ok := opts.State.PickProcess(); ok {
			pid = proc.ProcessID
			processName = proc.ProcessName
			user = proc.Username
			opts.State.RemoveProcess(pid)
		}
	}

	data := append(subjectFields(r, opts),
		EventDataField{Name: "Status", Value: "0x0"},
		EventDataField{Name: "ProcessId", Value: pid},
		EventDataField{Name: "ProcessName", Value: processName},
	)

	msg := fmt.Sprintf("A process has exited.\n\nSubject:\n\tAccount Name:\t\t%s\n\nProcess Information:\n\tProcess ID:\t\t%s\n\tProcess Name:\t\t%s",
		user, pid, processName)
	return data, msg
}

func generateServiceInstalled(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	serviceNames := []string{
		"WindowsDefender", "BITS", "wuauserv", "Spooler",
		"TermService", "WinRM", "W32Time", "CryptSvc",
	}
	svcName := serviceNames[r.Intn(len(serviceNames))] // #nosec G404
	svcPath := fmt.Sprintf(`C:\Windows\System32\%s.exe`, svcName)

	serviceTypes := []string{"0x10", "0x20", "0x110"}
	startTypes := []string{"0x2", "0x3", "0x4"}

	data := append(subjectFields(r, opts),
		EventDataField{Name: "ServiceName", Value: svcName},
		EventDataField{Name: "ServiceFileName", Value: svcPath},
		EventDataField{Name: "ServiceType", Value: serviceTypes[r.Intn(len(serviceTypes))]},  // #nosec G404
		EventDataField{Name: "ServiceStartType", Value: startTypes[r.Intn(len(startTypes))]}, // #nosec G404
		EventDataField{Name: "ServiceAccount", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, PickUsername(r, opts.Usernames))},
	)

	msg := fmt.Sprintf("A service was installed in the system.\n\nSubject:\n\tAccount Name:\t\t%s\n\nService Information:\n\tService Name:\t\t%s\n\tService File Name:\t%s",
		data[1].Value, svcName, svcPath)
	return data, msg
}
