//go:build embed_library

package embeddedlibrary

import (
	"io/fs"
	"testing"
)

// TestEmbeddedLibraryPopulated proves an embed_library build ships a
// non-empty library, guarding the embed seam against embedding nothing (PIPE-1445).
func TestEmbeddedLibraryPopulated(t *testing.T) {
	fsys := FS()

	var files int
	if err := fs.WalkDir(fsys, ".", func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files++
		}
		return nil
	}); err != nil {
		t.Fatalf("walk embedded FS: %v", err)
	}
	if files == 0 {
		t.Fatal("embedded data library is empty: the embed_library build embedded no files")
	}

	// Spot-check a couple of known packages resolve, so an embed that
	// captures only stray top-level files still fails.
	for _, pkg := range []string{"apache", "cisco"} {
		if _, err := fs.Stat(fsys, pkg); err != nil {
			t.Errorf("expected embedded package %q to resolve: %v", pkg, err)
		}
	}
}
