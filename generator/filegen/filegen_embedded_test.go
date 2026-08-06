//go:build embed_library

package filegen_test

import (
	"context"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/filegen"
	"github.com/observiq/blitz/generator/filegen/embeddedlibrary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockConsumer is a minimal embed.LogConsumer that captures records for
// assertion. (The in-package mockConsumer is unexported; this is the
// external-test mirror.)
type mockConsumer struct {
	mu      sync.Mutex
	records []embed.LogRecord
}

func (m *mockConsumer) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, records...)
	return nil
}

func (m *mockConsumer) snapshot() []embed.LogRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]embed.LogRecord, len(m.records))
	copy(out, m.records)
	return out
}

// TestFilegen_EmbeddedLibrarySource exercises the embedded-library
// backend end-to-end: filegen.New is constructed with embeddedlibrary.FS()
// and a "package:" source, runs briefly, and asserts records flow.
// The test depends on the data library containing the "syslog_generic"
// entry (which is part of the canonical data_library/ at the repo root
// and the snapshot under generator/filegen/embeddedlibrary/data_library/).
func TestFilegen_EmbeddedLibrarySource(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := &mockConsumer{}

	gen, err := filegen.New(
		logger,
		1,                   // workers
		10*time.Millisecond, // rate
		"package:syslog_generic",
		true, // cache enabled
		0,    // cache TTL
		consumer,
		embeddedlibrary.FS(),
		embed.NopTelemetry(),
	)
	require.NoError(t, err)

	require.NoError(t, gen.Start(context.Background()))

	require.Eventually(t,
		func() bool { return len(consumer.snapshot()) >= 3 },
		2*time.Second, 20*time.Millisecond,
		"expected at least 3 records from embedded library",
	)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(stopCtx))

	records := consumer.snapshot()
	assert.GreaterOrEqual(t, len(records), 3)
	for _, rec := range records {
		assert.NotEmpty(t, rec.Message, "record from embedded library should have a non-empty Message")
	}
}

// TestFilegen_BareNameFallsBackToLibrary checks that a bare-name source
// (no slash, no "package:" prefix) that doesn't match a disk entry
// resolves against the supplied data library FS. This is the
// backward-compat path for configs that reference data library entries
// by bare name.
func TestFilegen_BareNameFallsBackToLibrary(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := &mockConsumer{}

	// Build a tiny synthetic library FS so the test doesn't depend on
	// the contents of the real data library.
	libFS := fstest.MapFS{
		"my_package/sample.log": &fstest.MapFile{
			Data: []byte("hello from the library"),
		},
	}

	gen, err := filegen.New(
		logger,
		1,
		10*time.Millisecond,
		"my_package", // bare name; not a disk path
		true,
		0,
		consumer,
		libFS,
		embed.NopTelemetry(),
	)
	require.NoError(t, err)

	require.NoError(t, gen.Start(context.Background()))

	require.Eventually(t,
		func() bool { return len(consumer.snapshot()) >= 1 },
		2*time.Second, 20*time.Millisecond,
		"expected at least 1 record from synthetic library FS",
	)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(stopCtx))

	records := consumer.snapshot()
	assert.GreaterOrEqual(t, len(records), 1)
	assert.Equal(t, "hello from the library", records[0].Message)
}

// TestFilegen_PackagePrefixSkipsDiskFallback verifies that a source with
// the explicit "package:" prefix is resolved ONLY against the library
// FS — a disk path matching the same name is never tried.
func TestFilegen_PackagePrefixSkipsDiskFallback(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := &mockConsumer{}

	// Library doesn't contain "missing_package"; with the package:
	// prefix, no disk fallback is attempted, so this MUST fail.
	libFS := fstest.MapFS{
		"other_package/x.log": &fstest.MapFile{Data: []byte("x")},
	}

	gen, err := filegen.New(
		logger,
		1,
		10*time.Millisecond,
		"package:missing_package",
		true,
		0,
		consumer,
		libFS,
		embed.NopTelemetry(),
	)
	require.NoError(t, err)

	err = gen.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing_package")
}
