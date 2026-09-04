package embeddedlibrary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestReleaseConfigShipsLibraryOnDisk asserts the packages populate the
// on-disk library rather than shipping an empty dir (PIPE-1445).
func TestReleaseConfigShipsLibraryOnDisk(t *testing.T) {
	data, err := os.ReadFile(findRepoFile(t, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}

	var cfg struct {
		NFPMs []struct {
			Contents []struct {
				Src  string `yaml:"src"`
				Dst  string `yaml:"dst"`
				Type string `yaml:"type"`
			} `yaml:"contents"`
		} `yaml:"nfpms"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse goreleaser config: %v", err)
	}
	if len(cfg.NFPMs) == 0 {
		t.Fatal("goreleaser config declares no nfpms")
	}

	const dst = "/usr/share/blitz/data_library"
	for _, n := range cfg.NFPMs {
		for _, c := range n.Contents {
			if c.Dst != dst {
				continue
			}
			if c.Type == "dir" {
				t.Errorf("nfpm ships %s as an empty type:dir; it must copy the library files", dst)
			}
			if !strings.Contains(c.Src, "data_library") {
				t.Errorf("nfpm %s src %q does not reference the data_library tree", dst, c.Src)
			}
			return
		}
	}
	t.Errorf("no nfpm content entry populates %s", dst)
}

// findRepoFile walks up from the test's working directory to locate name
// at the repository root.
func findRepoFile(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return filepath.Join(dir, name)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find %s walking up from %s", name, dir)
		}
		dir = parent
	}
}
