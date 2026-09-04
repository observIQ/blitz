package datagen

import "testing"

func TestParseOSType(t *testing.T) {
	cases := map[string]struct {
		in      string
		want    OSType
		wantErr bool
	}{
		"linux":        {"linux", OSLinux, false},
		"windows":      {"windows", OSWindows, false},
		"macos":        {"macos", OSMacOS, false},
		"darwin alias": {"darwin", OSMacOS, false},
		"upper+space":  {"  MacOS ", OSMacOS, false},
		"unsupported":  {"freebsd", "", true},
		"empty":        {"", "", true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseOSType(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseOSType(%q): want error, got nil", c.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOSType(%q): unexpected error: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("ParseOSType(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestOSTypeFromGOOS(t *testing.T) {
	cases := map[string]OSType{
		"linux":   OSLinux,
		"windows": OSWindows,
		"darwin":  OSMacOS,
		"freebsd": OSType("freebsd"),
		"aix":     OSType("aix"),
	}
	for goos, want := range cases {
		if got := OSTypeFromGOOS(goos); got != want {
			t.Errorf("OSTypeFromGOOS(%q) = %q, want %q", goos, got, want)
		}
	}
}

func TestSemconvOSType(t *testing.T) {
	cases := map[OSType]string{
		OSLinux:           "linux",
		OSWindows:         "windows",
		OSMacOS:           "darwin",
		OSType("freebsd"): "freebsd",
	}
	for os, want := range cases {
		if got := os.SemconvOSType(); got != want {
			t.Errorf("%q.SemconvOSType() = %q, want %q", os, got, want)
		}
	}
}
