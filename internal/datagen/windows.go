package datagen

// Windows-specific data pools for event log generation.
var (
	// WindowsServices is a pool of real Windows service names for SCM events.
	WindowsServices = NewPool(
		"wuauserv", "BITS", "Dnscache", "LanmanServer", "LanmanWorkstation",
		"Spooler", "W32Time", "WinRM", "TermService", "WinDefend",
		"EventLog", "CryptSvc", "Dhcp", "TrkWks", "IISADMIN",
		"MSSQLSERVER", "SQLSERVERAGENT", "W3SVC", "WAS", "Netlogon",
		"NTDS", "DNS", "DFSR", "KDC",
		"gpsvc", "Schedule", "ProfSvc", "Themes", "AudioSrv",
	)

	// WindowsServiceDisplayNames is a pool of Windows service display names.
	WindowsServiceDisplayNames = NewPool(
		"Windows Update", "Background Intelligent Transfer Service",
		"DNS Client", "Server", "Workstation",
		"Print Spooler", "Windows Time", "Windows Remote Management",
		"Remote Desktop Services", "Windows Defender Antivirus Service",
		"Windows Event Log", "Cryptographic Services",
		"DHCP Client", "Distributed Link Tracking Client",
		"IIS Admin Service", "SQL Server (MSSQLSERVER)",
		"SQL Server Agent", "World Wide Web Publishing Service",
		"Windows Process Activation Service", "Net Logon",
		"Active Directory Domain Services", "DNS Server",
		"DFS Replication", "Net Logon", "Kerberos Key Distribution Center",
		"Group Policy Client", "Task Scheduler",
		"User Profile Service", "Themes", "Windows Audio",
	)

	// WindowsProcessPaths is a pool of common Windows process paths.
	WindowsProcessPaths = NewPool(
		`C:\Windows\System32\svchost.exe`,
		`C:\Windows\System32\lsass.exe`,
		`C:\Windows\System32\services.exe`,
		`C:\Windows\System32\csrss.exe`,
		`C:\Windows\System32\winlogon.exe`,
		`C:\Windows\System32\wininit.exe`,
		`C:\Windows\System32\dwm.exe`,
		`C:\Windows\System32\taskhostw.exe`,
		`C:\Windows\System32\conhost.exe`,
		`C:\Windows\System32\dllhost.exe`,
		`C:\Windows\System32\mmc.exe`,
		`C:\Windows\System32\cmd.exe`,
		`C:\Windows\System32\powershell.exe`,
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		`C:\Windows\explorer.exe`,
		`C:\Program Files\Windows Defender\MsMpEng.exe`,
		`C:\Windows\System32\spoolsv.exe`,
		`C:\Windows\System32\SearchIndexer.exe`,
		`C:\Windows\System32\wbem\WmiPrvSE.exe`,
		`C:\Windows\System32\msiexec.exe`,
	)

	// WindowsRegistryPaths is a pool of common registry paths for object access events.
	WindowsRegistryPaths = NewPool(
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`,
		`HKLM\SYSTEM\CurrentControlSet\Services`,
		`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`,
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer`,
		`HKLM\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`,
		`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`HKLM\SYSTEM\CurrentControlSet\Control\SecurityProviders`,
		`HKCU\Software\Microsoft\Office`,
		`HKLM\SOFTWARE\Microsoft\Windows Defender`,
		`HKLM\SYSTEM\CurrentControlSet\Control\Lsa`,
	)

	// WindowsTaskPaths is a pool of common scheduled task paths.
	WindowsTaskPaths = NewPool(
		`\Microsoft\Windows\UpdateOrchestrator\Schedule Scan`,
		`\Microsoft\Windows\WindowsUpdate\Scheduled Start`,
		`\Microsoft\Windows\Defrag\ScheduledDefrag`,
		`\Microsoft\Windows\Diagnosis\Scheduled`,
		`\Microsoft\Windows\DiskCleanup\SilentCleanup`,
		`\Microsoft\Windows\Maintenance\WinSAT`,
		`\Microsoft\Windows\SystemRestore\SR`,
		`\Microsoft\Windows\Time Synchronization\SynchronizeTime`,
		`\Microsoft\Windows\TaskScheduler\Regular Maintenance`,
		`\Microsoft\Windows\Servicing\StartComponentCleanup`,
	)
)
