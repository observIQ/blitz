package datagen

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
