package build

var (
	Version        string
	BuildTime      string
	BuildPlatform  string
	BinaryPlatform string
	Commit         string
)

type Info struct {
	Version        string `json:"version"`
	BuildTime      string `json:"buildTime"`
	BuildPlatform  string `json:"buildPlatform"`
	BinaryPlatform string `json:"binaryPlatform"`
	Commit         string `json:"commit"`
}

func GetInfo() Info {
	return Info{
		Version:        getValue(Version, "unknown"),
		BuildTime:      getValue(BuildTime, "unknown"),
		BuildPlatform:  getValue(BuildPlatform, "unknown"),
		BinaryPlatform: getValue(BinaryPlatform, "unknown"),
		Commit:         getValue(Commit, "unknown"),
	}
}

func getValue(v, defaultValue string) string {
	if v == "" {
		return defaultValue
	}
	return v
}
