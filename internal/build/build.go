package build

var (
	Version   string
	BuildTime string
	Platform  string
	Commit    string
)

type Info struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
	Platform  string `json:"platform"`
	Commit    string `json:"commit"`
}

func GetInfo() Info {
	return Info{
		Version:   getValue(Version, "unknown"),
		BuildTime: getValue(BuildTime, "unknown"),
		Platform:  getValue(Platform, "unknown"),
		Commit:    getValue(Commit, "unknown"),
	}
}

func getValue(v, defaultValue string) string {
	if v == "" {
		return defaultValue
	}
	return v
}
