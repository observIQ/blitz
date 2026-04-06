package catalog

import (
	"fmt"
	"math/rand"
)

var faultingApps = []string{
	"explorer.exe", "svchost.exe", "chrome.exe", "msedge.exe",
	"outlook.exe", "winword.exe", "excel.exe", "Teams.exe",
	"sqlservr.exe", "w3wp.exe", "iisexpress.exe",
}

var faultingModules = []string{
	"ntdll.dll", "kernelbase.dll", "msvcrt.dll", "user32.dll",
	"gdi32.dll", "combase.dll", "ole32.dll", "clr.dll",
}

var msiProducts = []string{
	"Microsoft Office Professional Plus 2021",
	"Microsoft Visual C++ 2022 Redistributable (x64)",
	"Microsoft .NET Framework 4.8",
	"Java 8 Update 381",
	"Adobe Acrobat Reader DC",
	"7-Zip 23.01 (x64)",
	"Google Chrome",
	"Notepad++ (64-bit x64)",
}

func init() {
	// Application Error (crash)
	Register(EventDefinition{
		Channel:  "Application",
		Provider: "Application Error",
		EventID:  1000,
		Level:    LevelError,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			app := faultingApps[r.Intn(len(faultingApps))]       // #nosec G404
			mod := faultingModules[r.Intn(len(faultingModules))] // #nosec G404
			data := []EventDataField{
				{Name: "AppName", Value: app},
				{Name: "AppVersion", Value: "10.0.19041.1"},
				{Name: "AppTimestamp", Value: RandomHexID(r, 4)},
				{Name: "ModName", Value: mod},
				{Name: "ModVersion", Value: "10.0.19041.1"},
				{Name: "ModTimestamp", Value: RandomHexID(r, 4)},
				{Name: "ExceptionCode", Value: "0xc0000005"},
				{Name: "FaultOffset", Value: RandomHexID(r, 4)},
				{Name: "ProcessId", Value: RandomProcessID(r)},
				{Name: "AppStartTime", Value: RandomHexID(r, 8)},
				{Name: "AppPath", Value: fmt.Sprintf(`C:\Windows\System32\%s`, app)},
				{Name: "ModPath", Value: fmt.Sprintf(`C:\Windows\System32\%s`, mod)},
				{Name: "ReportId", Value: RandomGUID(r)},
			}
			return data, fmt.Sprintf("Faulting application name: %s, version: 10.0.19041.1, faulting module name: %s, exception code: 0xc0000005", app, mod)
		},
	})

	// Application Hang
	Register(EventDefinition{
		Channel:  "Application",
		Provider: "Application Hang",
		EventID:  1002,
		Level:    LevelError,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			app := faultingApps[r.Intn(len(faultingApps))] // #nosec G404
			data := []EventDataField{
				{Name: "AppName", Value: app},
				{Name: "AppVersion", Value: "10.0.19041.1"},
				{Name: "ProcessId", Value: RandomProcessID(r)},
				{Name: "StartTime", Value: RandomHexID(r, 8)},
				{Name: "TerminationTime", Value: "4294967295"},
				{Name: "AppPath", Value: fmt.Sprintf(`C:\Windows\System32\%s`, app)},
				{Name: "ReportId", Value: RandomGUID(r)},
			}
			return data, fmt.Sprintf("The program %s stopped interacting with Windows and was closed.", app)
		},
	})

	// .NET Runtime unhandled exception
	Register(EventDefinition{
		Channel:  "Application",
		Provider: ".NET Runtime",
		EventID:  1026,
		Level:    LevelError,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			exceptions := []string{
				"System.NullReferenceException",
				"System.InvalidOperationException",
				"System.OutOfMemoryException",
				"System.IO.FileNotFoundException",
				"System.StackOverflowException",
			}
			exc := exceptions[r.Intn(len(exceptions))] // #nosec G404
			data := []EventDataField{
				{Name: "param1", Value: fmt.Sprintf("Application: app.exe\nFramework Version: v4.0.30319\nDescription: The process was terminated due to an unhandled exception.\nException Info: %s", exc)},
			}
			return data, fmt.Sprintf("Application: app.exe\nFramework Version: v4.0.30319\nDescription: The process was terminated due to an unhandled exception.\nException Info: %s", exc)
		},
	})

	// MSI Installer events
	for _, ev := range []struct {
		id    int
		level EventLevel
		msg   string
	}{
		{1033, LevelInformation, "installed"},
		{1034, LevelInformation, "removed"},
		{1035, LevelInformation, "updated"},
		{11707, LevelInformation, "completed successfully"},
		{11708, LevelError, "failed"},
	} {
		ev := ev
		Register(EventDefinition{
			Channel:  "Application",
			Provider: "MsiInstaller",
			EventID:  ev.id,
			Level:    ev.level,
			MinRole:  RoleWorkstation,
			Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
				product := msiProducts[r.Intn(len(msiProducts))] // #nosec G404
				user := PickUsername(r, opts.Usernames)
				data := []EventDataField{
					{Name: "ProductName", Value: product},
					{Name: "ProductVersion", Value: "1.0.0"},
					{Name: "Manufacturer", Value: "Microsoft Corporation"},
					{Name: "UserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
					{Name: "UserName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
				}
				return data, fmt.Sprintf("Windows Installer %s product. Product Name: %s. User: %s\\%s.",
					ev.msg, product, opts.DomainName, user)
			},
		})
	}

	// VSS events
	Register(EventDefinition{
		Channel:  "Application",
		Provider: "VSS",
		EventID:  8193,
		Level:    LevelError,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			data := []EventDataField{
				{Name: "ErrorContext", Value: "DeviceIoControl"},
				{Name: "ErrorCode", Value: "0x80042302"},
				{Name: "Operation", Value: "CreateShadowCopy"},
			}
			return data, "Volume Shadow Copy Service error: Unexpected error calling routine DeviceIoControl. hr = 0x80042302."
		},
	})

	// ESENT events
	Register(EventDefinition{
		Channel:  "Application",
		Provider: "ESENT",
		EventID:  326,
		Level:    LevelInformation,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			dbs := []string{
				`C:\Windows\system32\ESE\srudb.dat`,
				`C:\ProgramData\Microsoft\Search\Data\Applications\Windows\Windows.edb`,
				`C:\Windows\system32\ESE\DomainController.dit`,
			}
			db := dbs[r.Intn(len(dbs))] // #nosec G404
			data := []EventDataField{
				{Name: "DatabaseName", Value: db},
				{Name: "ProcessId", Value: RandomProcessID(r)},
			}
			return data, fmt.Sprintf("svchost (%s) The database engine attached a database (%s).", data[1].Value, db)
		},
	})

	// User Profiles Service
	Register(EventDefinition{
		Channel:  "Application",
		Provider: "Microsoft-Windows-User Profiles Service",
		EventID:  1530,
		Level:    LevelWarning,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			user := PickUsername(r, opts.Usernames)
			data := []EventDataField{
				{Name: "Detail", Value: fmt.Sprintf(`1 user registry handles leaked from \Registry\User\%s`, RandomSID(r, "S-1-5-21-0-0-0"))},
			}
			return data, fmt.Sprintf("Windows detected your registry file is still in use by other applications or services. The file will be unloaded now. User: %s.", user)
		},
	})
}
