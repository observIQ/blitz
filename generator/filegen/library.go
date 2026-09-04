package filegen

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/observiq/blitz/generator/filegen/embeddedlibrary"
)

// libraryProbePaths lists the on-disk locations checked for the library, in
// priority order: env override, cwd, in-repo, install path.
func libraryProbePaths() []string {
	return []string{
		os.Getenv("BLITZ_DATA_LIBRARY_DIR"),
		"data_library",
		"generator/filegen/embeddedlibrary/data_library",
		"/usr/share/blitz/data_library",
	}
}

// diskLibrary returns the first existing probe directory as an fs.FS and its
// path, or (nil, "") when none exist.
func diskLibrary() (fs.FS, string) {
	for _, p := range libraryProbePaths() {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return os.DirFS(p), p
		}
	}
	return nil, ""
}

// overlayLibrary layers disk over embedded, returning whichever is present,
// or nil when neither is.
func overlayLibrary(disk, embedded fs.FS) fs.FS {
	switch {
	case disk != nil && embedded != nil:
		return overlayFS{disk: disk, base: embedded}
	case disk != nil:
		return disk
	default:
		return embedded
	}
}

// hasContent reports whether fsys has at least one entry at its root.
func hasContent(fsys fs.FS) bool {
	if fsys == nil {
		return false
	}
	entries, err := fs.ReadDir(fsys, ".")
	return err == nil && len(entries) > 0
}

// Library is a resolved data library for the `blitz library` commands: the
// on-disk layer, the embedded layer, and helpers over their overlay.
type Library struct {
	disk     fs.FS
	embedded fs.FS
	diskPath string
}

// ResolveLibrary resolves the data library the CLI operates on: the first
// existing on-disk probe path overlaid on the embedded library.
func ResolveLibrary() Library {
	disk, path := diskLibrary()
	return Library{disk: disk, embedded: embeddedlibrary.FS(), diskPath: path}
}

// PackageInfo names a top-level library package and which layers hold it.
type PackageInfo struct {
	Name       string
	OnDisk     bool
	OnEmbedded bool
}

// Overrides reports whether the on-disk copy shadows an embedded one.
func (p PackageInfo) Overrides() bool { return p.OnDisk && p.OnEmbedded }

// FS returns the overlay of disk over embedded.
func (l Library) FS() fs.FS { return overlayLibrary(l.disk, l.embedded) }

// ActiveSource reports where the library resolves from: the disk path (noting
// when it overrides an embedded copy), "embedded", or "" when none is present.
func (l Library) ActiveSource() string {
	switch {
	case l.disk != nil && hasContent(l.embedded):
		return l.diskPath + " (overriding embedded)"
	case l.disk != nil:
		return l.diskPath
	case hasContent(l.embedded):
		return "embedded"
	default:
		return ""
	}
}

// Packages lists the union of top-level packages across both layers, sorted,
// marking which layers hold each (so callers can flag overrides).
func (l Library) Packages() ([]PackageInfo, error) {
	byName := map[string]*PackageInfo{}
	mark := func(fsys fs.FS, set func(*PackageInfo)) {
		if fsys == nil {
			return
		}
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := byName[e.Name()]
			if p == nil {
				p = &PackageInfo{Name: e.Name()}
				byName[e.Name()] = p
			}
			set(p)
		}
	}
	mark(l.disk, func(p *PackageInfo) { p.OnDisk = true })
	mark(l.embedded, func(p *PackageInfo) { p.OnEmbedded = true })

	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]PackageInfo, 0, len(names))
	for _, n := range names {
		out = append(out, *byName[n])
	}
	return out, nil
}

// FileInfo names a file within a package and which layers hold it.
type FileInfo struct {
	Name       string // path relative to the package directory
	OnDisk     bool
	OnEmbedded bool
}

// Overrides reports whether the on-disk file shadows an embedded one.
func (f FileInfo) Overrides() bool { return f.OnDisk && f.OnEmbedded }

// Files lists the files under pkg across both layers, sorted, marking which
// layers hold each. It errors when the package is absent from both.
func (l Library) Files(pkg string) ([]FileInfo, error) {
	byName := map[string]*FileInfo{}
	collect := func(fsys fs.FS, set func(*FileInfo)) {
		if fsys == nil {
			return
		}
		_ = fs.WalkDir(fsys, pkg, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel := strings.TrimPrefix(p, pkg+"/")
			f := byName[rel]
			if f == nil {
				f = &FileInfo{Name: rel}
				byName[rel] = f
			}
			set(f)
			return nil
		})
	}
	collect(l.disk, func(f *FileInfo) { f.OnDisk = true })
	collect(l.embedded, func(f *FileInfo) { f.OnEmbedded = true })
	if len(byName) == 0 {
		return nil, fmt.Errorf("package %q not found in the data library", pkg)
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]FileInfo, 0, len(names))
	for _, n := range names {
		out = append(out, *byName[n])
	}
	return out, nil
}

// Search returns packages whose name contains term (case-insensitive).
func (l Library) Search(term string) ([]PackageInfo, error) {
	all, err := l.Packages()
	if err != nil {
		return nil, err
	}
	term = strings.ToLower(term)
	var out []PackageInfo
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Name), term) {
			out = append(out, p)
		}
	}
	return out, nil
}

// Show returns the concatenated contents of pkg's files, resolved through the
// overlay so on-disk copies win.
func (l Library) Show(pkg string) (string, error) {
	files, err := l.Files(pkg)
	if err != nil {
		return "", err
	}
	fsys := l.FS()
	var b strings.Builder
	for _, f := range files {
		data, err := fs.ReadFile(fsys, pkg+"/"+f.Name)
		if err != nil {
			return "", fmt.Errorf("read %s/%s: %w", pkg, f.Name, err)
		}
		b.Write(data)
		if n := len(data); n > 0 && data[n-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

// DiffEntry is one file the on-disk library changes relative to embedded.
type DiffEntry struct {
	Path   string
	Status string // "override" or "added"
}

// Diff lists how the on-disk library differs from the embedded baseline:
// files in both with differing content ("override") and disk-only files
// ("added"). It is empty when either layer is absent.
func (l Library) Diff() ([]DiffEntry, error) {
	if l.disk == nil || !hasContent(l.embedded) {
		return nil, nil
	}
	var out []DiffEntry
	err := fs.WalkDir(l.disk, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		diskData, rerr := fs.ReadFile(l.disk, p)
		if rerr != nil {
			return rerr
		}
		embData, eerr := fs.ReadFile(l.embedded, p)
		switch {
		case eerr != nil:
			out = append(out, DiffEntry{Path: p, Status: "added"})
		case !bytes.Equal(diskData, embData):
			out = append(out, DiffEntry{Path: p, Status: "override"})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Extract copies pkg's overlay-resolved files to dest/data_library/<pkg>/.
func (l Library) Extract(pkg, dest string) error {
	return extractTree(l.FS(), pkg, dest)
}

// ExtractAll copies the whole overlay-resolved library to dest/data_library/.
func (l Library) ExtractAll(dest string) error {
	return extractTree(l.FS(), ".", dest)
}

// extractTree writes every file under root in fsys to dest/data_library/,
// preserving the library-relative path.
func extractTree(fsys fs.FS, root, dest string) error {
	if fsys == nil {
		return fmt.Errorf("no data library to extract")
	}
	return fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, rerr := fs.ReadFile(fsys, p)
		if rerr != nil {
			return rerr
		}
		out := filepath.Join(dest, "data_library", filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o600)
	})
}
