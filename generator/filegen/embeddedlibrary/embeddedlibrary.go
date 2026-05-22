// Package embeddedlibrary ships the filegen data library as files baked
// into the Go binary via //go:embed, gated by the embed_library build tag.
//
// Standalone blitz CLI does NOT import this package — it reads the
// data_library/ files from disk at runtime so users can edit them
// without a recompile. Library consumers (e.g. the OTel
// telemetrygeneratorreceiver) that want the data library bundled into
// their binary import this package and pass FS() to filegen.New.
//
// Build tag: this package only embeds files when built with
// `-tags embed_library`. Without the tag, FS() returns an empty
// filesystem and no embed directive is compiled, so bare `go build`,
// `go test ./...`, `go vet`, and IDE language servers work in this
// repo without baking ~8 MB of data into every binary. Embedded
// consumers add `-tags embed_library` to opt in.
//
// The canonical location for data_library/ is THIS package directory
// (./generator/filegen/embeddedlibrary/data_library/) — there is no
// separate root-level copy. The standalone CLI reads `./data_library/`
// from the process cwd at runtime; release tarballs, nfpms, and
// docker images stage the files at that path. From a fresh clone the
// CLI's `libraryFS` falls back to the in-repo canonical path so
// `./blitz` from repo root Just Works without a staging step.
package embeddedlibrary

import "io/fs"

// FS returns the data_library filesystem. Without the embed_library
// build tag this is an empty fs.FS; with the tag it is the embedded
// snapshot of data_library/. Library consumers pass the result to
// filegen.New to operate the file generator against the bundled data
// library instead of disk.
//
// Walking the returned FS lists entries relative to the data_library
// root (e.g. "syslog_generic/file.log"), matching the on-disk layout
// that the standalone CLI reads.
func FS() fs.FS { return libraryFS }
