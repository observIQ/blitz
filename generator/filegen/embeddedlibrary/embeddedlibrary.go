// Package embeddedlibrary ships the filegen data library as files baked
// into the Go binary via //go:embed, gated by the embed_library build tag.
//
// Standalone blitz CLI does NOT import this package — it reads the
// repo-root data_library/ from disk at runtime so users can edit files
// without a recompile. Library consumers (e.g. the OTel
// telemetrygeneratorreceiver) that want the data library bundled into
// their binary import this package and pass FS() to filegen.New.
//
// Build tag: this package only embeds files when built with
// `-tags embed_library`. Without the tag, FS() returns an empty
// filesystem and no embed directive is compiled, so bare `go build`,
// `go test ./...`, `go vet`, and IDE language servers work in this
// repo without first materializing the snapshot. Embedded consumers
// must add `-tags embed_library` to their build, and their build
// pipeline must run `make sync-embedded-library` first to materialize
// data_library/ into this package's directory.
//
// The repo-root data_library/ is the single source of truth.
// 'make sync-embedded-library' copies it into
// generator/filegen/embeddedlibrary/data_library as a build artifact
// (gitignored). Building with -tags embed_library then //go:embeds it.
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
