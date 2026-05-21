// Package runtime is the shared lifecycle core that both cli.Runner and
// embed.Runner will be thin wrappers around.
//
// The CLI Runner today tangles three concerns: orchestration (start
// workers, route records, stop cleanly), process-level concerns
// (signals, YAML config), and Output wiring. Only the first is
// reusable for embed.
//
// Subsequent PRs in PIPE-975 will populate this package with the shared
// orchestration code. PR #1 ships the skeleton so downstream PRs have a
// stable import path to migrate against.
package runtime
