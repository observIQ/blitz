package datagen

import "math/rand"

// ServiceStartType represents the startup type of a Windows/Linux service.
type ServiceStartType string

const (
	StartAutomatic ServiceStartType = "Automatic"
	StartManual    ServiceStartType = "Manual"
	StartDisabled  ServiceStartType = "Disabled"
	StartBoot      ServiceStartType = "Boot"
	StartSystem    ServiceStartType = "System"
)

// ServiceIdentity represents a service running on a system.
type ServiceIdentity struct {
	Name        string           // "wuauserv"
	DisplayName string           // "Windows Update"
	BinaryPath  string           // "C:\Windows\System32\svchost.exe -k netsvcs"
	StartType   ServiceStartType // Automatic, Manual, etc.
	Account     string           // "LocalSystem", "NT AUTHORITY\NETWORK SERVICE", or a UPN
	SystemRef   string           // back-reference: hostname of owning system
}

// Service template definitions.
type serviceTemplate struct {
	name        string
	displayName string
	binaryPath  string
	startType   ServiceStartType
	account     string
}

var dcServices = []serviceTemplate{
	{"NTDS", "Active Directory Domain Services", `C:\Windows\System32\ntdsa.dll`, StartAutomatic, "LocalSystem"},
	{"DNS", "DNS Server", `C:\Windows\System32\dns.exe`, StartAutomatic, "LocalSystem"},
	{"KDC", "Kerberos Key Distribution Center", `C:\Windows\System32\lsass.exe`, StartAutomatic, "LocalSystem"},
	{"Netlogon", "Net Logon", `C:\Windows\System32\netlogon.dll`, StartAutomatic, "LocalSystem"},
	{"DFS", "DFS Replication", `C:\Windows\System32\dfsr.exe`, StartAutomatic, `NT AUTHORITY\NETWORK SERVICE`},
	{"W32Time", "Windows Time", `C:\Windows\System32\w32time.dll`, StartAutomatic, `NT AUTHORITY\LOCAL SERVICE`},
	{"EventLog", "Windows Event Log", `C:\Windows\System32\wevtsvc.dll`, StartAutomatic, `NT AUTHORITY\LOCAL SERVICE`},
	{"CertSvc", "Active Directory Certificate Services", `C:\Windows\System32\certsrv.exe`, StartAutomatic, "LocalSystem"},
}

var windowsServerServices = []serviceTemplate{
	{"W3SVC", "World Wide Web Publishing Service", `C:\Windows\System32\inetsrv\iisw3adm.dll`, StartAutomatic, "LocalSystem"},
	{"MSSQLSERVER", "SQL Server (MSSQLSERVER)", `C:\Program Files\Microsoft SQL Server\MSSQL16.MSSQLSERVER\MSSQL\Binn\sqlservr.exe`, StartAutomatic, `NT SERVICE\MSSQLSERVER`},
	{"Spooler", "Print Spooler", `C:\Windows\System32\spoolsv.exe`, StartAutomatic, "LocalSystem"},
	{"WinRM", "Windows Remote Management (WS-Management)", `C:\Windows\System32\winrm.cmd`, StartAutomatic, `NT AUTHORITY\NETWORK SERVICE`},
	{"WinDefend", "Windows Defender Antivirus Service", `C:\Program Files\Windows Defender\MsMpEng.exe`, StartAutomatic, "LocalSystem"},
	{"EventLog", "Windows Event Log", `C:\Windows\System32\wevtsvc.dll`, StartAutomatic, `NT AUTHORITY\LOCAL SERVICE`},
	{"W32Time", "Windows Time", `C:\Windows\System32\w32time.dll`, StartAutomatic, `NT AUTHORITY\LOCAL SERVICE`},
	{"CryptSvc", "Cryptographic Services", `C:\Windows\System32\cryptsvc.dll`, StartAutomatic, `NT AUTHORITY\NETWORK SERVICE`},
	{"wuauserv", "Windows Update", `C:\Windows\System32\wuaueng.dll`, StartAutomatic, "LocalSystem"},
	{"BITS", "Background Intelligent Transfer Service", `C:\Windows\System32\qmgr.dll`, StartManual, "LocalSystem"},
}

var windowsWorkstationServices = []serviceTemplate{
	{"WinDefend", "Windows Defender Antivirus Service", `C:\Program Files\Windows Defender\MsMpEng.exe`, StartAutomatic, "LocalSystem"},
	{"EventLog", "Windows Event Log", `C:\Windows\System32\wevtsvc.dll`, StartAutomatic, `NT AUTHORITY\LOCAL SERVICE`},
	{"AudioSrv", "Windows Audio", `C:\Windows\System32\audiosrv.dll`, StartAutomatic, `NT AUTHORITY\LOCAL SERVICE`},
	{"Themes", "Themes", `C:\Windows\System32\themeservice.dll`, StartAutomatic, "LocalSystem"},
	{"Schedule", "Task Scheduler", `C:\Windows\System32\schedsvc.dll`, StartAutomatic, "LocalSystem"},
}

var linuxServerServices = []serviceTemplate{
	{"sshd", "OpenSSH server daemon", "/usr/sbin/sshd", StartAutomatic, "root"},
	{"nginx", "A high performance web server", "/usr/sbin/nginx", StartAutomatic, "www-data"},
	{"postgresql", "PostgreSQL RDBMS", "/usr/lib/postgresql/16/bin/postgres", StartAutomatic, "postgres"},
	{"docker", "Docker Application Container Engine", "/usr/bin/dockerd", StartAutomatic, "root"},
	{"cron", "Regular background program processing daemon", "/usr/sbin/cron", StartAutomatic, "root"},
	{"rsyslog", "System Logging Service", "/usr/sbin/rsyslogd", StartAutomatic, "syslog"},
	{"systemd-journald", "Journal Service", "/lib/systemd/systemd-journald", StartAutomatic, "root"},
	{"prometheus-node-exporter", "Prometheus Node Exporter", "/usr/bin/prometheus-node-exporter", StartAutomatic, "prometheus"},
}

var linuxWorkstationServices = []serviceTemplate{
	{"sshd", "OpenSSH server daemon", "/usr/sbin/sshd", StartAutomatic, "root"},
	{"cron", "Regular background program processing daemon", "/usr/sbin/cron", StartAutomatic, "root"},
	{"NetworkManager", "Network Manager", "/usr/sbin/NetworkManager", StartAutomatic, "root"},
}

// GenerateServicesForSystem returns services appropriate for the given OS and role.
//
// Domain controllers always receive every entry in dcServices: NTDS, DNS,
// KDC, Netlogon and the rest are AD-critical and a DC with any of them
// missing would be internally incoherent for downstream simulators. For
// other roles, services are emitted as a random subset of the template list
// so different hosts of the same role have variation.
func GenerateServicesForSystem(r *rand.Rand, os OSType, role SystemRole, hostname string) []*ServiceIdentity {
	var templates []serviceTemplate
	subsetEligible := true

	switch {
	case role == RoleDC:
		templates = dcServices
		subsetEligible = false // DCs always run their full service set
	case os == OSWindows && role == RoleServer:
		templates = windowsServerServices
	case os == OSWindows && role == RoleWorkstation:
		templates = windowsWorkstationServices
	case os == OSLinux && role == RoleServer:
		templates = linuxServerServices
	case os == OSLinux && role == RoleWorkstation:
		templates = linuxWorkstationServices
	default:
		// Router or macOS — minimal services
		templates = []serviceTemplate{
			{"sshd", "OpenSSH server daemon", "/usr/sbin/sshd", StartAutomatic, "root"},
		}
	}

	count := len(templates)
	if subsetEligible && count > 5 {
		// Pick a random subset (at least 3, up to all)
		count = 3 + r.Intn(count-2) // #nosec G404
		if count > len(templates) {
			count = len(templates)
		}
	}

	// Shuffle and take first count
	indices := r.Perm(len(templates))
	services := make([]*ServiceIdentity, count)
	for i := 0; i < count; i++ {
		tmpl := templates[indices[i]]
		services[i] = &ServiceIdentity{
			Name:        tmpl.name,
			DisplayName: tmpl.displayName,
			BinaryPath:  tmpl.binaryPath,
			StartType:   tmpl.startType,
			Account:     tmpl.account,
			SystemRef:   hostname,
		}
	}

	return services
}
