package filegen

import (
	"io/fs"
	"sort"
)

// overlayFS layers a disk fs.FS over an embedded one: disk wins per path,
// base fills what disk lacks, so an on-disk library overrides and extends
// the embedded one (PIPE-1445). Either layer may be nil. It implements
// StatFS and ReadDirFS so WalkDir sees the union of both.
type overlayFS struct {
	disk fs.FS // override/extension layer; checked first
	base fs.FS // embedded fallback layer
}

var (
	_ fs.FS        = overlayFS{}
	_ fs.StatFS    = overlayFS{}
	_ fs.ReadDirFS = overlayFS{}
)

// Open resolves disk-first, then base.
func (o overlayFS) Open(name string) (fs.File, error) {
	if o.disk != nil {
		if f, err := o.disk.Open(name); err == nil {
			return f, nil
		}
	}
	if o.base != nil {
		return o.base.Open(name)
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// Stat resolves disk-first, then base.
func (o overlayFS) Stat(name string) (fs.FileInfo, error) {
	if o.disk != nil {
		if fi, err := fs.Stat(o.disk, name); err == nil {
			return fi, nil
		}
	}
	if o.base != nil {
		return fs.Stat(o.base, name)
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// ReadDir merges both layers, disk winning per name, sorted per the
// fs.ReadDir contract. Missing from both layers is a not-exist error.
func (o overlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries := map[string]fs.DirEntry{}
	found := false

	// Disk first so its entries win on collision.
	if o.disk != nil {
		if des, err := fs.ReadDir(o.disk, name); err == nil {
			found = true
			for _, de := range des {
				entries[de.Name()] = de
			}
		}
	}
	if o.base != nil {
		if des, err := fs.ReadDir(o.base, name); err == nil {
			found = true
			for _, de := range des {
				if _, ok := entries[de.Name()]; !ok {
					entries[de.Name()] = de
				}
			}
		}
	}
	if !found {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]fs.DirEntry, 0, len(names))
	for _, n := range names {
		out = append(out, entries[n])
	}
	return out, nil
}
