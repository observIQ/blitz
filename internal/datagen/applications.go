package datagen

import (
	"math/rand"

	"go.uber.org/zap"
)

// ApplicationIdentity represents installed software on a system.
type ApplicationIdentity struct {
	Name        string // "Microsoft SQL Server 2019"
	Version     string // "15.0.4322.2"
	Vendor      string // "Microsoft Corporation"
	InstallPath string // "C:\Program Files\Microsoft SQL Server"
	InstallDate string // "2024-06-15"
	SystemRef   string // back-reference: hostname of owning system
}

// appTemplate defines an application template.
type appTemplate struct {
	name        string
	version     string
	vendor      string
	installPath string
}

var windowsServerApps = []appTemplate{
	{"Internet Information Services", "10.0.20348", "Microsoft Corporation", `C:\Windows\System32\inetsrv`},
	{"Microsoft SQL Server 2019", "15.0.4322.2", "Microsoft Corporation", `C:\Program Files\Microsoft SQL Server`},
	{".NET Runtime 8.0", "8.0.1", "Microsoft Corporation", `C:\Program Files\dotnet`},
	{"Microsoft Exchange Server 2019", "15.2.1118.40", "Microsoft Corporation", `C:\Program Files\Microsoft\Exchange Server`},
	{"Microsoft SharePoint Server 2019", "16.0.10396.20000", "Microsoft Corporation", `C:\Program Files\Microsoft Office Servers`},
	{"Windows Server Backup", "10.0.20348", "Microsoft Corporation", `C:\Windows\System32`},
	{"Microsoft Visual C++ 2019 Redistributable", "14.29.30133", "Microsoft Corporation", `C:\Program Files\Microsoft Visual Studio`},
}

var linuxServerApps = []appTemplate{
	{"nginx", "1.24.0", "Nginx Inc.", "/usr/sbin/nginx"},
	{"Apache HTTP Server", "2.4.58", "Apache Software Foundation", "/usr/sbin/apache2"},
	{"PostgreSQL", "16.1", "PostgreSQL Global Development Group", "/usr/lib/postgresql/16"},
	{"MySQL", "8.0.35", "Oracle Corporation", "/usr/sbin/mysqld"},
	{"Docker Engine", "24.0.7", "Docker Inc.", "/usr/bin/docker"},
	{"OpenJDK Runtime Environment", "17.0.9", "Eclipse Adoptium", "/usr/lib/jvm/java-17-openjdk"},
	{"Node.js", "20.10.0", "OpenJS Foundation", "/usr/bin/node"},
	{"Redis", "7.2.3", "Redis Ltd.", "/usr/bin/redis-server"},
}

var windowsWorkstationApps = []appTemplate{
	{"Microsoft Office 365 ProPlus", "16.0.17126.20132", "Microsoft Corporation", `C:\Program Files\Microsoft Office`},
	{"Google Chrome", "120.0.6099.130", "Google LLC", `C:\Program Files\Google\Chrome`},
	{"Mozilla Firefox", "121.0", "Mozilla Foundation", `C:\Program Files\Mozilla Firefox`},
	{"Visual Studio Code", "1.85.1", "Microsoft Corporation", `C:\Users\AppData\Local\Programs\Microsoft VS Code`},
	{"Slack", "4.35.126", "Slack Technologies", `C:\Users\AppData\Local\slack`},
	{"Microsoft Teams", "1.6.00.36062", "Microsoft Corporation", `C:\Users\AppData\Local\Microsoft\Teams`},
	{"Zoom Workplace", "5.17.0", "Zoom Video Communications", `C:\Program Files\Zoom`},
	{"Adobe Acrobat Reader", "23.008.20470", "Adobe Inc.", `C:\Program Files\Adobe\Acrobat Reader DC`},
}

var linuxWorkstationApps = []appTemplate{
	{"Mozilla Firefox", "121.0", "Mozilla Foundation", "/usr/lib/firefox"},
	{"Google Chrome", "120.0.6099.130", "Google LLC", "/opt/google/chrome"},
	{"LibreOffice", "7.6.4", "The Document Foundation", "/usr/lib/libreoffice"},
	{"Visual Studio Code", "1.85.1", "Microsoft Corporation", "/usr/share/code"},
	{"Slack", "4.35.126", "Slack Technologies", "/usr/lib/slack"},
	{"GIMP", "2.10.36", "The GIMP Team", "/usr/bin/gimp"},
	{"VLC media player", "3.0.20", "VideoLAN", "/usr/bin/vlc"},
	{"Thunderbird", "115.6.0", "Mozilla Foundation", "/usr/lib/thunderbird"},
}

var macosWorkstationApps = []appTemplate{
	{"Safari", "17.2.1", "Apple Inc.", "/Applications/Safari.app"},
	{"Google Chrome", "120.0.6099.130", "Google LLC", "/Applications/Google Chrome.app"},
	{"Microsoft Office 365", "16.80", "Microsoft Corporation", "/Applications/Microsoft 365"},
	{"Visual Studio Code", "1.85.1", "Microsoft Corporation", "/Applications/Visual Studio Code.app"},
	{"Slack", "4.35.126", "Slack Technologies", "/Applications/Slack.app"},
	{"Zoom Workplace", "5.17.0", "Zoom Video Communications", "/Applications/zoom.us.app"},
	{"Xcode", "15.2", "Apple Inc.", "/Applications/Xcode.app"},
	{"Homebrew", "4.2.0", "Homebrew", "/opt/homebrew"},
}

var dcApps = []appTemplate{
	{"Active Directory Domain Services", "10.0.20348", "Microsoft Corporation", `C:\Windows\System32`},
	{"Active Directory Certificate Services", "10.0.20348", "Microsoft Corporation", `C:\Windows\System32\certsrv`},
	{"DNS Server", "10.0.20348", "Microsoft Corporation", `C:\Windows\System32\dns.exe`},
	{"DHCP Server", "10.0.20348", "Microsoft Corporation", `C:\Windows\System32`},
	{"Remote Server Administration Tools", "10.0.20348", "Microsoft Corporation", `C:\Windows\System32`},
	{"Group Policy Management Console", "10.0.20348", "Microsoft Corporation", `C:\Windows\System32`},
}

// installDates provides deterministic install date strings.
var installDates = NewPool(
	"2024-01-15", "2024-02-20", "2024-03-10", "2024-04-05",
	"2024-05-18", "2024-06-22", "2024-07-30", "2024-08-14",
	"2024-09-01", "2024-10-12", "2024-11-25", "2024-12-03",
)

// GenerateApplicationsForSystem returns applications appropriate for the given OS and role.
//
// Supported combinations:
//   - Any role == RoleDC → DC application set (Windows-based AD services).
//   - OSWindows + RoleServer → Windows server applications.
//   - OSWindows + RoleWorkstation → Windows workstation applications.
//   - OSLinux + (RoleServer | RoleRouter) → Linux server applications.
//   - OSLinux + RoleWorkstation → Linux workstation applications.
//   - OSMacOS + RoleWorkstation → macOS workstation applications.
//
// Any other (os, role) combination is unsupported: the function returns an
// empty slice and emits a Warn on the injected logger so misconfigurations
// surface in logs without panicking the caller. A nil logger disables the
// warning (no global-logger fallback), per the embed-contract logging rules
// (PIPE-1067).
func GenerateApplicationsForSystem(r *rand.Rand, os OSType, role SystemRole, hostname string, logger *zap.Logger) []*ApplicationIdentity {
	if logger == nil {
		logger = zap.NewNop()
	}
	var templates []appTemplate

	switch {
	case role == RoleDC:
		templates = dcApps
	case os == OSWindows && role == RoleServer:
		templates = windowsServerApps
	case os == OSWindows && role == RoleWorkstation:
		templates = windowsWorkstationApps
	case os == OSLinux && (role == RoleServer || role == RoleRouter):
		templates = linuxServerApps
	case os == OSLinux && role == RoleWorkstation:
		templates = linuxWorkstationApps
	case os == OSMacOS && role == RoleWorkstation:
		templates = macosWorkstationApps
	default:
		logger.Warn("GenerateApplicationsForSystem: unsupported os/role combination — returning empty application set",
			zap.String("os", string(os)),
			zap.String("role", string(role)),
			zap.String("hostname", hostname),
		)
		return []*ApplicationIdentity{}
	}

	// Pick a random subset (at least 2, up to all)
	count := len(templates)
	if count > 3 {
		count = 2 + r.Intn(count-1) // #nosec G404
		if count > len(templates) {
			count = len(templates)
		}
	}

	indices := r.Perm(len(templates))
	apps := make([]*ApplicationIdentity, count)
	for i := 0; i < count; i++ {
		tmpl := templates[indices[i]]
		apps[i] = &ApplicationIdentity{
			Name:        tmpl.name,
			Version:     tmpl.version,
			Vendor:      tmpl.vendor,
			InstallPath: tmpl.installPath,
			InstallDate: installDates.Random(r),
			SystemRef:   hostname,
		}
	}

	return apps
}
