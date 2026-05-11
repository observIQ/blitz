package datagen

// ApplicationIdentity represents installed software on a system.
type ApplicationIdentity struct {
	Name        string // "Microsoft SQL Server 2019"
	Version     string // "15.0.4322.2"
	Vendor      string // "Microsoft Corporation"
	InstallPath string // "C:\Program Files\Microsoft SQL Server"
	InstallDate string // "2024-06-15"
	SystemRef   string // back-reference: hostname of owning system
}
