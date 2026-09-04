package filegen

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// walkFiles returns the sorted list of file paths under root in fsys.
func walkFiles(t *testing.T, fsys fs.FS, root string) []string {
	t.Helper()
	var got []string
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if !d.IsDir() {
			got = append(got, p)
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(got)
	return got
}

func readFile(t *testing.T, fsys fs.FS, name string) string {
	t.Helper()
	b, err := fs.ReadFile(fsys, name)
	require.NoError(t, err)
	return string(b)
}

// A package present only in the embedded (base) layer resolves through the overlay.
func TestOverlayFS_EmbeddedOnly(t *testing.T) {
	base := fstest.MapFS{
		"pkg/a.log": {Data: []byte("base-a")},
		"pkg/b.log": {Data: []byte("base-b")},
	}
	o := overlayFS{disk: nil, base: base}

	require.Equal(t, []string{"pkg/a.log", "pkg/b.log"}, walkFiles(t, o, "pkg"))
	require.Equal(t, "base-a", readFile(t, o, "pkg/a.log"))
}

// A package present only on disk resolves through the overlay.
func TestOverlayFS_DiskOnly(t *testing.T) {
	disk := fstest.MapFS{
		"pkg/a.log": {Data: []byte("disk-a")},
	}
	o := overlayFS{disk: disk, base: nil}

	require.Equal(t, []string{"pkg/a.log"}, walkFiles(t, o, "pkg"))
	require.Equal(t, "disk-a", readFile(t, o, "pkg/a.log"))
}

// A file present in both layers returns the on-disk contents (override),
// and the walk lists it once.
func TestOverlayFS_DiskOverridesEmbedded(t *testing.T) {
	disk := fstest.MapFS{"pkg/a.log": {Data: []byte("disk-a")}}
	base := fstest.MapFS{"pkg/a.log": {Data: []byte("base-a")}}
	o := overlayFS{disk: disk, base: base}

	require.Equal(t, []string{"pkg/a.log"}, walkFiles(t, o, "pkg"))
	require.Equal(t, "disk-a", readFile(t, o, "pkg/a.log"))
}

// A package split across both layers returns the union, with disk winning
// per file: shared files come from disk, disk-only and base-only files both appear.
func TestOverlayFS_UnionDiskWinsPerFile(t *testing.T) {
	disk := fstest.MapFS{
		"pkg/shared.log": {Data: []byte("disk-shared")},
		"pkg/disk.log":   {Data: []byte("disk-only")},
	}
	base := fstest.MapFS{
		"pkg/shared.log": {Data: []byte("base-shared")},
		"pkg/base.log":   {Data: []byte("base-only")},
	}
	o := overlayFS{disk: disk, base: base}

	require.Equal(t, []string{"pkg/base.log", "pkg/disk.log", "pkg/shared.log"}, walkFiles(t, o, "pkg"))
	require.Equal(t, "disk-shared", readFile(t, o, "pkg/shared.log"))
	require.Equal(t, "disk-only", readFile(t, o, "pkg/disk.log"))
	require.Equal(t, "base-only", readFile(t, o, "pkg/base.log"))
}

// libraryFS() builds the overlay from the on-disk probe (here via
// BLITZ_DATA_LIBRARY_DIR) layered over the embedded FS passed to New,
// so the wired resolution path exercises the same disk-over-embedded
// behavior as the overlay unit tests (PIPE-1445).
func TestLibraryFS_OverlaysDiskOverEmbedded(t *testing.T) {
	diskDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(diskDir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(diskDir, "pkg", "shared.log"), []byte("disk-shared"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(diskDir, "pkg", "disk.log"), []byte("disk-only"), 0o644))
	t.Setenv("BLITZ_DATA_LIBRARY_DIR", diskDir)

	base := fstest.MapFS{
		"pkg/shared.log": {Data: []byte("base-shared")},
		"pkg/base.log":   {Data: []byte("base-only")},
	}
	g := &FileLogGenerator{dataLibrary: base}

	require.Equal(t, []string{"pkg/base.log", "pkg/disk.log", "pkg/shared.log"}, walkFiles(t, g.libraryFS(), "pkg"))
	require.Equal(t, "disk-shared", readFile(t, g.libraryFS(), "pkg/shared.log"))
	require.Equal(t, "base-only", readFile(t, g.libraryFS(), "pkg/base.log"))
}
