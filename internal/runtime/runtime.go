package runtime

import (
	"github.com/observiq/blitz/embed"
	"go.uber.org/zap"
)

// Runtime is the shared lifecycle core extracted from cli.Runner. It
// owns module orchestration: starting modules, routing records to
// consumers, and orderly shutdown.
//
// Runtime is not constructed directly by external callers — cli.Runner
// and embed.Runner each wrap a Runtime with their own process-level or
// host-level concerns.
//
// The struct is intentionally minimal in PR #1 — fields and methods are
// added in PR #12 (CLI Runner migration) and PR #13 (embed.New) as
// those PRs need them.
type Runtime struct {
	logger  *zap.Logger
	modules []embed.ProducerModule
}

// New returns a Runtime configured with the given logger and modules.
// PR #1 ships the constructor signature; orchestration methods land in
// PR #12.
func New(logger *zap.Logger, modules []embed.ProducerModule) *Runtime {
	return &Runtime{
		logger:  logger,
		modules: modules,
	}
}
