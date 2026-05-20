//go:build !embed_library

package embeddedlibrary

import "io/fs"

// libraryFS is the empty stub used when the package is built without
// the embed_library tag. Bare `go build` / `go test` in this repo
// compiles this file and never touches //go:embed, so the package
// builds cleanly even when the data_library snapshot has not been
// materialized into this directory.
var libraryFS fs.FS = emptyFS{}

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }
