//go:build embed_library

package embeddedlibrary

import (
	"embed"
	"io/fs"
)

//go:embed all:data_library
var embeddedRoot embed.FS

// libraryFS is the embedded snapshot used when the package is built
// with `-tags embed_library`. The build pipeline (Makefile target
// sync-embedded-library, also invoked from goreleaser before-hooks)
// materializes the repo-root data_library/ into this package's
// directory before compilation; //go:embed picks it up here.
var libraryFS fs.FS = func() fs.FS {
	sub, err := fs.Sub(embeddedRoot, "data_library")
	if err != nil {
		// fs.Sub only fails for an invalid name; "data_library" is a
		// fixed string we control, so this is unreachable.
		panic("embeddedlibrary: fs.Sub failed for data_library: " + err.Error())
	}
	return sub
}()
