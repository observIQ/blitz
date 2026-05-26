package catalog

import (
	"fmt"
	"math/rand"
)

func init() {
	// Service Control Manager events
	scmProvider := "Service Control Manager"
	scmGUID := "{555908d1-a6d7-4695-8e1e-26931d2012f4}"

	scmEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{7000, LevelError, generateSCMServiceFailed},
		{7009, LevelError, generateSCMServiceTimeout},
		{7023, LevelError, generateSCMServiceError},
		{7031, LevelError, generateSCMServiceCrash},
		{7036, LevelInformation, generateSCMServiceStateChange},
		{7040, LevelInformation, generateSCMServiceStartTypeChanged},
		{7045, LevelInformation, generateSCMNewServiceInstalled},
	}

	for _, ev := range scmEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      "System",
			Provider:     scmProvider,
			ProviderGUID: scmGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}

	// Kernel/Power events
	Register(EventDefinition{
		Channel:      "System",
		Provider:     "Microsoft-Windows-Kernel-Power",
		ProviderGUID: "{331c3b3a-2005-44c2-ac5e-77220c37d6b4}",
		EventID:      41,
		Level:        LevelCritical,
		MinRole:      RoleWorkstation,
		Generate: func(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			data := []EventDataField{
				{Name: "BugcheckCode", Value: "0"},
				{Name: "BugcheckParameter1", Value: "0x0"},
				{Name: "BugcheckParameter2", Value: "0x0"},
				{Name: "BugcheckParameter3", Value: "0x0"},
				{Name: "BugcheckParameter4", Value: "0x0"},
				{Name: "SleepInProgress", Value: "0"},
				{Name: "PowerButtonTimestamp", Value: "0"},
			}
			return data, "The system has rebooted without cleanly shutting down first. This error could be caused if the system stopped responding, crashed, or lost power unexpectedly."
		},
	})

	Register(EventDefinition{
		Channel:      "System",
		Provider:     "Microsoft-Windows-Kernel-Power",
		ProviderGUID: "{331c3b3a-2005-44c2-ac5e-77220c37d6b4}",
		EventID:      42,
		Level:        LevelInformation,
		MinRole:      RoleWorkstation,
		Generate: func(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			return []EventDataField{
				{Name: "TargetState", Value: "4"},
				{Name: "EffectiveState", Value: "4"},
			}, "The system is entering sleep."
		},
	})

	// EventLog events
	for _, ev := range []struct {
		id    int
		level EventLevel
		msg   string
	}{
		{6005, LevelInformation, "The Event log service was started."},
		{6006, LevelInformation, "The Event log service was stopped."},
		{6008, LevelError, "The previous system shutdown was unexpected."},
		{6013, LevelInformation, "The system uptime is 86400 seconds."},
	} {
		ev := ev
		Register(EventDefinition{
			Channel:  "System",
			Provider: "EventLog",
			EventID:  ev.id,
			Level:    ev.level,
			MinRole:  RoleWorkstation,
			Generate: func(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
				return nil, ev.msg
			},
		})
	}

	// Disk events
	Register(EventDefinition{
		Channel:  "System",
		Provider: "Disk",
		EventID:  7,
		Level:    LevelError,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			devices := []string{`\Device\Harddisk0\DR0`, `\Device\Harddisk1\DR1`}
			dev := devices[r.Intn(len(devices))] // #nosec G404
			return []EventDataField{
				{Name: "DeviceName", Value: dev},
			}, fmt.Sprintf("The device, %s, has a bad block.", dev)
		},
	})

	Register(EventDefinition{
		Channel:  "System",
		Provider: "Disk",
		EventID:  11,
		Level:    LevelError,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			devices := []string{`\Device\Harddisk0\DR0`, `\Device\Harddisk1\DR1`}
			dev := devices[r.Intn(len(devices))] // #nosec G404
			return []EventDataField{
				{Name: "DeviceName", Value: dev},
			}, fmt.Sprintf("The driver detected a controller error on %s.", dev)
		},
	})

	// Time Service
	Register(EventDefinition{
		Channel:      "System",
		Provider:     "Microsoft-Windows-Time-Service",
		ProviderGUID: "{06edcfeb-0fd0-4e53-acca-a6f8bbf81bcb}",
		EventID:      134,
		Level:        LevelWarning,
		MinRole:      RoleWorkstation,
		Generate: func(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			return []EventDataField{
				{Name: "TimeSource", Value: "time.windows.com,0x9"},
			}, "NtpClient was unable to set a domain peer to use as a time source because of discovery error. NtpClient will try again in 15 minutes."
		},
	})

	// DNS Client
	Register(EventDefinition{
		Channel:  "System",
		Provider: "Microsoft-Windows-DNS-Client",
		EventID:  1014,
		Level:    LevelWarning,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			names := []string{"dc01.contoso.com", "mail.contoso.com", "www.contoso.com"}
			name := names[r.Intn(len(names))] // #nosec G404
			return []EventDataField{
				{Name: "QueryName", Value: name},
			}, fmt.Sprintf("Name resolution for the name %s timed out after none of the configured DNS servers responded.", name)
		},
	})

	// DCOM
	Register(EventDefinition{
		Channel:  "System",
		Provider: "Microsoft-Windows-DistributedCOM",
		EventID:  10016,
		Level:    LevelWarning,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			user := PickUsername(r, opts.Usernames)
			data := []EventDataField{
				{Name: "param1", Value: "application-specific"},
				{Name: "param2", Value: "Local"},
				{Name: "param3", Value: "Activation"},
				{Name: "param4", Value: RandomGUID(r)},
				{Name: "param5", Value: RandomGUID(r)},
				{Name: "param6", Value: fmt.Sprintf("%s\\%s", opts.DomainName, user)},
				{Name: "param7", Value: RandomSID(r, "S-1-5-21-0-0-0")},
			}
			return data, fmt.Sprintf("The application-specific permission settings do not grant Local Activation permission for the COM Server application with CLSID %s to the user %s\\%s.",
				data[3].Value, opts.DomainName, user)
		},
	})

	// Shutdown
	Register(EventDefinition{
		Channel:  "System",
		Provider: "USER32",
		EventID:  1074,
		Level:    LevelInformation,
		MinRole:  RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			user := PickUsername(r, opts.Usernames)
			reasons := []string{"Operating System: Upgrade (Planned)", "No title for this reason could be found", "Operating System: Service pack (Planned)"}
			reason := reasons[r.Intn(len(reasons))] // #nosec G404
			data := []EventDataField{
				{Name: "param1", Value: `C:\Windows\System32\shutdown.exe`},
				{Name: "param2", Value: opts.Computer},
				{Name: "param3", Value: reason},
				{Name: "param4", Value: "0x80020003"},
				{Name: "param5", Value: "restart"},
				{Name: "param6", Value: ""},
				{Name: "param7", Value: fmt.Sprintf("%s\\%s", opts.DomainName, user)},
			}
			return data, fmt.Sprintf("The process C:\\Windows\\System32\\shutdown.exe (%s) has initiated the restart of computer %s on behalf of user %s\\%s for the following reason: %s",
				opts.Computer, opts.Computer, opts.DomainName, user, reason)
		},
	})
}

func generateSCMServiceFailed(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	svcNames := []string{"wuauserv", "BITS", "Dnscache", "Spooler", "W32Time", "WinRM"}
	svc := svcNames[r.Intn(len(svcNames))] // #nosec G404
	data := []EventDataField{
		{Name: "param1", Value: svc},
	}
	return data, fmt.Sprintf("The %s service failed to start due to the following error: The service did not respond to the start or control request in a timely fashion.", svc)
}

func generateSCMServiceTimeout(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	svcNames := []string{"wuauserv", "BITS", "Dnscache", "Spooler"}
	svc := svcNames[r.Intn(len(svcNames))] // #nosec G404
	data := []EventDataField{
		{Name: "param1", Value: svc},
		{Name: "param2", Value: "30000"},
	}
	return data, fmt.Sprintf("A timeout was reached (30000 milliseconds) while waiting for the %s service to connect.", svc)
}

func generateSCMServiceError(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	svcNames := []string{"wuauserv", "BITS", "Spooler", "TermService"}
	svc := svcNames[r.Intn(len(svcNames))] // #nosec G404
	data := []EventDataField{
		{Name: "param1", Value: svc},
		{Name: "param2", Value: "%%2147944122"},
	}
	return data, fmt.Sprintf("The %s service terminated with the following error: The specified service does not exist as an installed service.", svc)
}

func generateSCMServiceCrash(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	svcNames := []string{"WinDefend", "Spooler", "BITS", "wuauserv"}
	svc := svcNames[r.Intn(len(svcNames))] // #nosec G404
	data := []EventDataField{
		{Name: "param1", Value: svc},
		{Name: "param2", Value: fmt.Sprintf("%d", r.Intn(5)+1)}, // #nosec G404
		{Name: "param3", Value: "60000"},
		{Name: "param4", Value: "Restart the service."},
	}
	return data, fmt.Sprintf("The %s service terminated unexpectedly. It has done this %s time(s). The following corrective action will be taken in 60000 milliseconds: Restart the service.",
		svc, data[1].Value)
}

func generateSCMServiceStateChange(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	svcNames := []string{"wuauserv", "BITS", "Dnscache", "Spooler", "W32Time", "WinRM", "WinDefend"}
	svc := svcNames[r.Intn(len(svcNames))] // #nosec G404
	states := []string{"running", "stopped"}
	state := states[r.Intn(len(states))] // #nosec G404
	data := []EventDataField{
		{Name: "param1", Value: svc},
		{Name: "param2", Value: state},
	}
	return data, fmt.Sprintf("The %s service entered the %s state.", svc, state)
}

func generateSCMServiceStartTypeChanged(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	svcNames := []string{"wuauserv", "BITS", "Spooler"}
	svc := svcNames[r.Intn(len(svcNames))] // #nosec G404
	startTypes := []string{"auto start", "demand start", "disabled"}
	oldType := startTypes[r.Intn(len(startTypes))] // #nosec G404
	newType := startTypes[r.Intn(len(startTypes))] // #nosec G404
	data := []EventDataField{
		{Name: "param1", Value: svc},
		{Name: "param2", Value: oldType},
		{Name: "param3", Value: newType},
	}
	return data, fmt.Sprintf("The start type of the %s service was changed from %s to %s.", svc, oldType, newType)
}

func generateSCMNewServiceInstalled(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	svcName := fmt.Sprintf("BlitzTestService%d", r.Intn(100)) // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "ServiceName", Value: svcName},
		{Name: "ImagePath", Value: fmt.Sprintf(`C:\Program Files\%s\service.exe`, svcName)},
		{Name: "ServiceType", Value: "user mode service"},
		{Name: "StartType", Value: "auto start"},
		{Name: "AccountName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("A service was installed in the system.\n\nService Name:\t%s\nService File Name:\tC:\\Program Files\\%s\\service.exe\nService Account:\t%s\\%s",
		svcName, svcName, opts.DomainName, user)
}
