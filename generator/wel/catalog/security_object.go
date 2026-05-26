package catalog

import (
	"fmt"
	"math/rand"
)

const (
	taskFileSystem    = 12800
	taskRegistry      = 12801
	taskSAMAccess     = 12802
	taskFileShare     = 12808
	taskDetailedShare = 12811
)

var (
	objectTypes = []string{"File", "Key", "Printer", "Process", "Thread", "Section", "Token", "Directory"}
	filePaths   = []string{
		`C:\Windows\System32\config\SAM`,
		`C:\Windows\System32\drivers\etc\hosts`,
		`C:\Users\Public\Documents\report.docx`,
		`C:\Program Files\Company\app.exe`,
		`C:\Windows\Temp\setup.log`,
		`\\?\GLOBALROOT\Device\HarddiskVolume2\Windows\System32\svchost.exe`,
	}
	sharePaths = []string{
		`\\*\ADMIN$`, `\\*\C$`, `\\*\IPC$`, `\\*\NETLOGON`, `\\*\SYSVOL`,
		`\\SERVER01\SharedDocs`, `\\SERVER01\Software`,
	}
	shareLocalPaths = []string{
		`C:\Windows`, `C:\`, ``, `C:\Windows\SYSVOL\sysvol`, `C:\Windows\SYSVOL\sysvol`,
		`D:\SharedDocs`, `D:\Software`,
	}
)

func init() {
	// Object access events
	for _, ev := range []struct {
		id   int
		task int
		kw   uint64
		gen  func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4656, taskFileSystem, keywordsAuditSuccess, generateHandleRequested},
		{4657, taskRegistry, keywordsAuditSuccess, generateRegistryValueModified},
		{4658, taskFileSystem, keywordsAuditSuccess, generateHandleClosed},
		{4660, taskFileSystem, keywordsAuditSuccess, generateObjectDeleted},
		{4663, taskFileSystem, keywordsAuditSuccess, generateObjectAccess},
		{4670, taskFileSystem, keywordsAuditSuccess, generatePermissionsChanged},
		{5140, taskFileShare, keywordsAuditSuccess, generateNetworkShareAccessed},
		{5145, taskDetailedShare, keywordsAuditSuccess, generateDetailedFileShare},
	} {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         ev.task,
			TaskName:     "Object Access",
			Keywords:     ev.kw,
			KeywordNames: keywordNames(ev.kw),
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateHandleRequested(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	objType := objectTypes[r.Intn(len(objectTypes))] // #nosec G404
	objName := filePaths[r.Intn(len(filePaths))]     // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectServer", Value: "Security"},
		EventDataField{Name: "ObjectType", Value: objType},
		EventDataField{Name: "ObjectName", Value: objName},
		EventDataField{Name: "HandleId", Value: RandomHexID(r, 4)},
		EventDataField{Name: "TransactionId", Value: RandomGUID(r)},
		EventDataField{Name: "AccessList", Value: "%%4416\n\t\t\t%%4423"},
		EventDataField{Name: "AccessMask", Value: RandomAccessMask(r)},
		EventDataField{Name: "PrivilegeList", Value: "-"},
		EventDataField{Name: "ProcessId", Value: RandomProcessID(r)},
		EventDataField{Name: "ProcessName", Value: `C:\Windows\System32\svchost.exe`},
		EventDataField{Name: "ResourceAttributes", Value: "-"},
	)
	msg := fmt.Sprintf("A handle to an object was requested.\n\nSubject:\n\tAccount Name:\t\t%s\n\nObject:\n\tObject Type:\t\t%s\n\tObject Name:\t\t%s",
		user, objType, objName)
	return data, msg
}

func generateRegistryValueModified(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	regPaths := []string{
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`,
		`HKLM\SYSTEM\CurrentControlSet\Services`,
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer`,
		`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`,
	}
	regPath := regPaths[r.Intn(len(regPaths))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectName", Value: regPath},
		EventDataField{Name: "ObjectValueName", Value: "TestValue"},
		EventDataField{Name: "HandleId", Value: RandomHexID(r, 4)},
		EventDataField{Name: "OperationType", Value: "%%1905"},
		EventDataField{Name: "OldValueType", Value: "%%1873"},
		EventDataField{Name: "OldValue", Value: ""},
		EventDataField{Name: "NewValueType", Value: "%%1873"},
		EventDataField{Name: "NewValue", Value: "TestData"},
		EventDataField{Name: "ProcessId", Value: RandomProcessID(r)},
		EventDataField{Name: "ProcessName", Value: `C:\Windows\regedit.exe`},
	)
	msg := fmt.Sprintf("A registry value was modified.\n\nSubject:\n\tAccount Name:\t\t%s\n\nObject:\n\tObject Name:\t\t%s",
		user, regPath)
	return data, msg
}

func generateHandleClosed(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectServer", Value: "Security"},
		EventDataField{Name: "HandleId", Value: RandomHexID(r, 4)},
		EventDataField{Name: "ProcessId", Value: RandomProcessID(r)},
		EventDataField{Name: "ProcessName", Value: `C:\Windows\System32\svchost.exe`},
	)
	msg := fmt.Sprintf("The handle to an object was closed.\n\nSubject:\n\tAccount Name:\t\t%s", data[1].Value)
	return data, msg
}

func generateObjectDeleted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectServer", Value: "Security"},
		EventDataField{Name: "HandleId", Value: RandomHexID(r, 4)},
		EventDataField{Name: "ProcessId", Value: RandomProcessID(r)},
		EventDataField{Name: "ProcessName", Value: `C:\Windows\System32\cmd.exe`},
		EventDataField{Name: "TransactionId", Value: RandomGUID(r)},
	)
	msg := fmt.Sprintf("An object was deleted.\n\nSubject:\n\tAccount Name:\t\t%s", data[1].Value)
	return data, msg
}

func generateObjectAccess(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	objName := filePaths[r.Intn(len(filePaths))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectServer", Value: "Security"},
		EventDataField{Name: "ObjectType", Value: "File"},
		EventDataField{Name: "ObjectName", Value: objName},
		EventDataField{Name: "HandleId", Value: RandomHexID(r, 4)},
		EventDataField{Name: "AccessList", Value: "%%4416"},
		EventDataField{Name: "AccessMask", Value: RandomAccessMask(r)},
		EventDataField{Name: "ProcessId", Value: RandomProcessID(r)},
		EventDataField{Name: "ProcessName", Value: `C:\Windows\explorer.exe`},
		EventDataField{Name: "ResourceAttributes", Value: "-"},
	)
	msg := fmt.Sprintf("An attempt was made to access an object.\n\nSubject:\n\tAccount Name:\t\t%s\n\nObject:\n\tObject Name:\t\t%s",
		user, objName)
	return data, msg
}

func generatePermissionsChanged(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	objName := filePaths[r.Intn(len(filePaths))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ObjectServer", Value: "Security"},
		EventDataField{Name: "ObjectType", Value: "File"},
		EventDataField{Name: "ObjectName", Value: objName},
		EventDataField{Name: "HandleId", Value: RandomHexID(r, 4)},
		EventDataField{Name: "OldSd", Value: "D:(A;;FA;;;BA)"},
		EventDataField{Name: "NewSd", Value: "D:(A;;FA;;;BA)(A;;0x1200a9;;;BU)"},
		EventDataField{Name: "ProcessId", Value: RandomProcessID(r)},
		EventDataField{Name: "ProcessName", Value: `C:\Windows\explorer.exe`},
	)
	msg := fmt.Sprintf("Permissions on an object were changed.\n\nSubject:\n\tAccount Name:\t\t%s\n\nObject:\n\tObject Name:\t\t%s",
		user, objName)
	return data, msg
}

func generateNetworkShareAccessed(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	idx := r.Intn(len(sharePaths)) // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ShareName", Value: sharePaths[idx]},
		EventDataField{Name: "ShareLocalPath", Value: shareLocalPaths[idx]},
		EventDataField{Name: "IpAddress", Value: PickIP(r, opts.IPs)},
		EventDataField{Name: "IpPort", Value: RandomPort(r)},
	)
	msg := fmt.Sprintf("A network share object was accessed.\n\nSubject:\n\tAccount Name:\t\t%s\n\nNetwork Information:\n\tShare Name:\t\t%s",
		user, sharePaths[idx])
	return data, msg
}

func generateDetailedFileShare(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	idx := r.Intn(len(sharePaths)) // #nosec G404
	relTarget := `Documents\report.docx`
	user := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "ShareName", Value: sharePaths[idx]},
		EventDataField{Name: "ShareLocalPath", Value: shareLocalPaths[idx]},
		EventDataField{Name: "RelativeTargetName", Value: relTarget},
		EventDataField{Name: "AccessMask", Value: RandomAccessMask(r)},
		EventDataField{Name: "AccessList", Value: "%%4416"},
		EventDataField{Name: "IpAddress", Value: PickIP(r, opts.IPs)},
		EventDataField{Name: "IpPort", Value: RandomPort(r)},
	)
	msg := fmt.Sprintf("A network share object was checked.\n\nSubject:\n\tAccount Name:\t\t%s\n\nShare:\n\tShare Name:\t\t%s\n\tRelative Target:\t%s",
		user, sharePaths[idx], relTarget)
	return data, msg
}
