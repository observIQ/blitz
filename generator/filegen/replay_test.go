package filegen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func messages(c *mockConsumer) []string {
	var out []string
	for _, r := range c.getWrites() {
		out = append(out, r.Message)
	}
	return out
}

func newReplay(t *testing.T, source string, consumer *mockConsumer) *FileLogGenerator {
	t.Helper()
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, source, true, 0, consumer, nil, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, gen.SetReplay())
	return gen
}

// waitDone waits for the tracker's count to be reached, failing instead of
// hanging the test binary when it never is.
func waitDone(t *testing.T, tracker *count.Tracker) {
	t.Helper()
	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("count tracker never reached its count")
	}
}

func stop(t *testing.T, gen *FileLogGenerator) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(ctx))
}

// TestReplay_EmitsEachLineOnceInOrder pins the replay contract: every
// non-empty line once, in file order, byte for byte, then nothing more.
func TestReplay_EmitsEachLineOnceInOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "first line\n\nsecond \t line, 100% hard-coded 2026-01-02T03:04:05Z\nthird\n")

	consumer := newMockConsumer()
	gen := newReplay(t, path, consumer)
	require.NoError(t, gen.Start(context.Background()))
	defer stop(t, gen)

	want := []string{"first line", "second \t line, 100% hard-coded 2026-01-02T03:04:05Z", "third"}
	require.Eventually(t, func() bool { return len(consumer.getWrites()) >= len(want) }, 5*time.Second, time.Millisecond)
	require.Never(t, func() bool { return len(consumer.getWrites()) > len(want) }, 100*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, want, messages(consumer))
}

// TestReplay_DirectoryInLexicalOrder pins that a directory replays file by
// file in lexical order, each file top to bottom.
func TestReplay_DirectoryInLexicalOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.log"), "b1\nb2\n")
	writeFile(t, filepath.Join(dir, "a.log"), "a1\na2\n")

	consumer := newMockConsumer()
	gen := newReplay(t, dir, consumer)
	require.NoError(t, gen.Start(context.Background()))
	defer stop(t, gen)

	want := []string{"a1", "a2", "b1", "b2"}
	require.Eventually(t, func() bool { return len(consumer.getWrites()) >= len(want) }, 5*time.Second, time.Millisecond)
	require.Never(t, func() bool { return len(consumer.getWrites()) > len(want) }, 100*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, want, messages(consumer))
}

// TestReplay_Count pins that ReplayCount reports the lines a replay emits, and
// zero in random mode, so the CLI can size the run's count tracker.
func TestReplay_Count(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\n\nb\nc\n")

	gen := newReplay(t, path, newMockConsumer())
	n, err := gen.ReplayCount()
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	random, err := New(zaptest.NewLogger(t), 1, time.Millisecond, path, true, 0, newMockConsumer(), nil, embed.NopTelemetry())
	require.NoError(t, err)
	n, err = random.ReplayCount()
	require.NoError(t, err)
	assert.Zero(t, n)
}

// TestReplay_StopsAtCountAndRestartsOnReset pins the finish path: a count
// below the line count stops the replay early, and a tracker reset (SIGUSR1)
// replays again from the first line.
func TestReplay_StopsAtCountAndRestartsOnReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\nb\nc\n")

	consumer := newMockConsumer()
	gen := newReplay(t, path, consumer)
	tracker := count.NewTracker(2)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	defer stop(t, gen)

	waitDone(t, tracker)
	require.Never(t, func() bool { return len(consumer.getWrites()) > 2 }, 100*time.Millisecond, 5*time.Millisecond)
	assert.Equal(t, []string{"a", "b"}, messages(consumer))

	tracker.Reset()
	require.Eventually(t, func() bool { return len(consumer.getWrites()) >= 4 }, 5*time.Second, time.Millisecond)
	assert.Equal(t, []string{"a", "b", "a", "b"}, messages(consumer))
}

// TestReplay_RestartsAfterEndOfInput pins that a replay sized to its full
// line count restarts from the first line on reset.
func TestReplay_RestartsAfterEndOfInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\nb\n")

	consumer := newMockConsumer()
	gen := newReplay(t, path, consumer)
	tracker := count.NewTracker(2)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	defer stop(t, gen)

	waitDone(t, tracker)
	tracker.Reset()
	require.Eventually(t, func() bool { return len(consumer.getWrites()) >= 4 }, 5*time.Second, time.Millisecond)
	assert.Equal(t, []string{"a", "b", "a", "b"}, messages(consumer))
}

// TestReplay_RejectsMultipleWorkers pins that replay is a single ordered
// stream.
func TestReplay_RejectsMultipleWorkers(t *testing.T) {
	gen, err := New(zaptest.NewLogger(t), 2, time.Millisecond, "/tmp/x.log", true, 0, newMockConsumer(), nil, embed.NopTelemetry())
	require.NoError(t, err)
	assert.ErrorContains(t, gen.SetReplay(), "workers")
}

// TestReplay_EmptyInputFails pins that a replay with nothing to emit fails at
// Start instead of running forever with no output.
func TestReplay_EmptyInputFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.log")
	writeFile(t, path, "\n\n")
	gen := newReplay(t, path, newMockConsumer())
	assert.ErrorContains(t, gen.Start(context.Background()), "no lines")
}

// TestReadFileLines_LongLine pins that a line past bufio.Scanner's default
// 64 KiB token limit is read whole instead of failing.
func TestReadFileLines_LongLine(t *testing.T) {
	long := strings.Repeat("x", 200*1024)
	path := filepath.Join(t.TempDir(), "long.log")
	writeFile(t, path, "short\n"+long+"\n")

	gen := newReplay(t, path, newMockConsumer())
	lines, err := gen.readFileLines(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"short", long}, lines)
}

// TestRandomMode_SeedIsDeterministic pins that a fixed seed reproduces the
// random mode's line sequence.
func TestRandomMode_SeedIsDeterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.log")
	var b strings.Builder
	for i := range 50 {
		b.WriteString(strings.Repeat("l", i+1) + "\n")
	}
	writeFile(t, path, b.String())

	run := func() []string {
		consumer := newMockConsumer()
		gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, path, true, 0, consumer, nil, embed.NopTelemetry())
		require.NoError(t, err)
		gen.SetSeed(42)
		tracker := count.NewTracker(20)
		gen.SetCountTracker(tracker)
		require.NoError(t, gen.Start(context.Background()))
		waitDone(t, tracker)
		require.Eventually(t, func() bool { return len(consumer.getWrites()) == 20 }, 5*time.Second, time.Millisecond)
		stop(t, gen)
		return messages(consumer)
	}
	assert.Equal(t, run(), run())
}

// TestGlobMatchingDirectories_ListsEachFileOnce pins that a glob matching
// several directories lists each directory's own files once, rather than
// re-globbing the whole pattern per matched directory.
func TestGlobMatchingDirectories_ListsEachFileOnce(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"logs_a", "logs_b"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o755))
		writeFile(t, filepath.Join(root, d, "x.log"), "line\n")
	}
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, filepath.Join(root, "logs_*"), true, 0, newMockConsumer(), nil, embed.NopTelemetry())
	require.NoError(t, err)
	files, err := gen.getFiles()
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(root, "logs_a", "x.log"), filepath.Join(root, "logs_b", "x.log")}, files)
}

// flakyConsumer fails its first n ConsumeLogs calls, then delivers.
type flakyConsumer struct {
	*mockConsumer
	mu    sync.Mutex
	fails int
}

func (f *flakyConsumer) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
	f.mu.Lock()
	if f.fails > 0 {
		f.fails--
		f.mu.Unlock()
		return errors.New("transient")
	}
	f.mu.Unlock()
	return f.mockConsumer.ConsumeLogs(ctx, records)
}

// TestReplay_RetryDoesNotSpendAPermit pins that a failed send gives its permit
// back and retries the same line, so a count sized to the input still
// delivers every line.
func TestReplay_RetryDoesNotSpendAPermit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\nb\nc\n")

	consumer := &flakyConsumer{mockConsumer: newMockConsumer(), fails: 2}
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, path, true, 0, consumer, nil, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, gen.SetReplay())
	tracker := count.NewTracker(3)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	defer stop(t, gen)

	require.Eventually(t, func() bool { return len(consumer.getWrites()) >= 3 }, 10*time.Second, time.Millisecond)
	assert.Equal(t, []string{"a", "b", "c"}, messages(consumer.mockConsumer))
}

// TestReplay_DirectorySkipsSubdirectories pins that a directory source lists
// only its files, so a subdirectory neither fails a replay nor counts as input.
func TestReplay_DirectorySkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.log"), "a1\n")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "b_sub"), 0o755))
	writeFile(t, filepath.Join(dir, "c.log"), "c1\n")

	gen := newReplay(t, dir, newMockConsumer())
	n, err := gen.ReplayCount()
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

// TestReplay_MissingSourceFails pins that a replay of a source that does not
// exist fails when its input is resolved, before anything starts.
func TestReplay_MissingSourceFails(t *testing.T) {
	gen := newReplay(t, filepath.Join(t.TempDir(), "missing.log"), newMockConsumer())
	_, err := gen.ReplayCount()
	assert.ErrorContains(t, err, "get files")
	assert.ErrorContains(t, gen.Start(context.Background()), "get files")
}

// TestReplay_MidRunResetRestartsAndFinishes pins that a reset landing in the
// middle of a replay (SIGUSR1) restarts it from the first line, so the
// refilled count is still reached and the run can end.
func TestReplay_MidRunResetRestartsAndFinishes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\nb\nc\nd\ne\n")

	consumer := newMockConsumer()
	gen, err := New(zaptest.NewLogger(t), 1, 5*time.Millisecond, path, true, 0, consumer, nil, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, gen.SetReplay())
	tracker := count.NewTracker(5)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	defer stop(t, gen)

	require.Eventually(t, func() bool { return len(consumer.getWrites()) >= 2 }, 5*time.Second, time.Millisecond)
	tracker.Reset()
	waitDone(t, tracker)

	got := messages(consumer)
	require.GreaterOrEqual(t, len(got), 5)
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, got[len(got)-5:],
		"the replay after the reset runs from the first line")
}

// hookConsumer runs onSend before delivering each record.
type hookConsumer struct {
	*mockConsumer
	onSend func(line string) error
}

func (h *hookConsumer) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
	if err := h.onSend(records[0].Message); err != nil {
		return err
	}
	return h.mockConsumer.ConsumeLogs(ctx, records)
}

// TestReplay_DoneWaitsForTheLastLine pins that the count completes only once
// the last line is delivered, so an exit-mode run cannot stop with the last
// line still failing.
func TestReplay_DoneWaitsForTheLastLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\nb\nc\n")

	failed := false
	consumer := &hookConsumer{mockConsumer: newMockConsumer(), onSend: func(line string) error {
		if line == "c" && !failed {
			failed = true
			return errors.New("transient")
		}
		return nil
	}}
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, path, true, 0, consumer, nil, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, gen.SetReplay())
	tracker := count.NewTracker(3)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	defer stop(t, gen)

	waitDone(t, tracker)
	assert.Equal(t, []string{"a", "b", "c"}, messages(consumer.mockConsumer))
}

// TestReplay_ResetDuringDeliveryRestarts pins that a reset landing while a
// line is in flight does not count that line toward the refilled count: the
// replay restarts from the first line and delivers a full pass.
func TestReplay_ResetDuringDeliveryRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\nb\nc\n")

	tracker := count.NewTracker(3)
	reset := false
	consumer := &hookConsumer{mockConsumer: newMockConsumer(), onSend: func(line string) error {
		if line == "b" && !reset {
			reset = true
			tracker.Reset()
		}
		return nil
	}}
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, path, true, 0, consumer, nil, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, gen.SetReplay())
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	defer stop(t, gen)

	// The reset replaces the done channel, so poll for the refilled count
	// rather than holding a channel taken before the reset.
	require.Eventually(t, func() bool { return tracker.Emitted() == 3 }, 5*time.Second, time.Millisecond)
	assert.Equal(t, []string{"a", "b", "a", "b", "c"}, messages(consumer.mockConsumer))
}

// TestReplay_DirectorySkipsSymlinkedDirectories pins that a symlink to a
// directory is skipped like a subdirectory.
func TestReplay_DirectorySkipsSymlinkedDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.log"), "a1\n")
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(dir, "b_link")))

	gen := newReplay(t, dir, newMockConsumer())
	n, err := gen.ReplayCount()
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

// TestReplay_UnreadableFileFails pins that a replay input that cannot be read
// fails when the input is resolved.
func TestReplay_UnreadableFileFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads files regardless of permissions")
	}
	path := filepath.Join(t.TempDir(), "locked.log")
	writeFile(t, path, "a\n")
	require.NoError(t, os.Chmod(path, 0o000))

	gen := newReplay(t, path, newMockConsumer())
	_, err := gen.ReplayCount()
	assert.ErrorContains(t, err, "open file")
}

// TestReplay_ReadsInputOnce pins that the input is read once: Start replays
// what ReplayCount counted, even if the file changed in between.
func TestReplay_ReadsInputOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\nb\n")
	consumer := newMockConsumer()
	gen := newReplay(t, path, consumer)
	n, err := gen.ReplayCount()
	require.NoError(t, err)
	require.Equal(t, 2, n)

	writeFile(t, path, "x\ny\nz\n")
	tracker := count.NewTracker(int64(n))
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	waitDone(t, tracker)
	stop(t, gen)
	assert.Equal(t, []string{"a", "b"}, messages(consumer))
}

// TestReplay_UnreadableDirectoryFails pins that a source directory that
// cannot be listed fails when the input is resolved.
func TestReplay_UnreadableDirectoryFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root lists directories regardless of permissions")
	}
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := newReplay(t, dir, newMockConsumer()).ReplayCount()
	assert.ErrorContains(t, err, "read directory")
}

// TestStopWhileRefused pins that a worker parked on a spent count stops
// promptly, in both modes.
func TestStopWhileRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\n")
	for _, replay := range []bool{false, true} {
		t.Run(fmt.Sprintf("replay=%v", replay), func(t *testing.T) {
			t.Parallel()
			stopWhileRefused(t, path, replay)
		})
	}
}

func stopWhileRefused(t *testing.T, path string, replay bool) {
	consumer := newMockConsumer()
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, path, true, 0, consumer, nil, embed.NopTelemetry())
	require.NoError(t, err)
	if replay {
		require.NoError(t, gen.SetReplay())
	}
	tracker := count.NewTracker(1)
	_, _, ok := tracker.Acquire() // spend the count, so the worker is refused
	require.True(t, ok)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	require.Never(t, func() bool { return len(consumer.getWrites()) > 0 }, 100*time.Millisecond, 5*time.Millisecond)
	stop(t, gen)
}

// TestReplay_StopsAtEndOfInput pins that a replay waiting at the end of its
// input for a reset stops promptly.
func TestReplay_StopsAtEndOfInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\n")
	consumer := newMockConsumer()
	gen := newReplay(t, path, consumer)
	tracker := count.NewTracker(5) // larger than the input, so the worker waits at the end
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	require.Eventually(t, func() bool { return len(consumer.getWrites()) == 1 }, 5*time.Second, time.Millisecond)
	// At a 1ms rate the worker reaches the end of the input many times over
	// this window, so Stop finds it waiting there.
	require.Never(t, func() bool { return len(consumer.getWrites()) > 1 }, 50*time.Millisecond, time.Millisecond)
	stop(t, gen)
}

// TestRandomMode_RefusedWorkerWakesOnReset pins that a random-mode worker
// refused a permit waits for the tracker, then generates again on Reset.
func TestRandomMode_RefusedWorkerWakesOnReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.log")
	writeFile(t, path, "a\n")
	consumer := newMockConsumer()
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, path, true, 0, consumer, nil, embed.NopTelemetry())
	require.NoError(t, err)
	tracker := count.NewTracker(1)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	t.Cleanup(func() { stop(t, gen) })

	for cycle := 1; cycle <= 2; cycle++ {
		waitDone(t, tracker)
		// At a 1ms rate the worker is refused many times over this window,
		// so Reset finds it waiting.
		require.Never(t, func() bool { return len(consumer.getWrites()) > cycle },
			50*time.Millisecond, time.Millisecond, "the refused worker wrote nothing more")
		tracker.Reset()
	}
}

// TestRandomMode_EmptyFilesKeepTheRate pins that skipping a file with no
// lines is not a failure, so a run of them does not back the worker off.
func TestRandomMode_EmptyFilesKeepTheRate(t *testing.T) {
	dir := t.TempDir()
	for i := range 20 {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("a%02d.log", i)), "\n")
	}
	writeFile(t, filepath.Join(dir, "z.log"), "line\n")
	consumer := newMockConsumer()
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, dir, true, 0, consumer, nil, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, gen.Start(context.Background()))
	t.Cleanup(func() { stop(t, gen) })

	// Backing off 1.5x per empty file would take about 6s to reach z.log.
	require.Eventually(t, func() bool { return len(consumer.getWrites()) > 0 }, 2*time.Second, time.Millisecond)
}

// TestReplay_GlobUnreadableDirectoryFails pins that a glob matching a
// directory replay cannot read fails at startup instead of dropping its lines.
func TestReplay_GlobUnreadableDirectoryFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root lists directories regardless of permissions")
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "a"), 0o700))
	writeFile(t, filepath.Join(root, "a", "1.log"), "l1\n")
	locked := filepath.Join(root, "b")
	require.NoError(t, os.Mkdir(locked, 0o700))
	writeFile(t, filepath.Join(locked, "2.log"), "l2\n")
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	_, err := newReplay(t, filepath.Join(root, "*"), newMockConsumer()).ReplayCount()
	assert.ErrorContains(t, err, "read directory")
}

// TestReplay_GlobBrokenMatchFails pins that a glob match replay cannot stat,
// such as a dangling symlink, fails at startup instead of dropping its lines.
func TestReplay_GlobBrokenMatchFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.log"), "l1\n")
	require.NoError(t, os.Symlink(filepath.Join(dir, "gone.log"), filepath.Join(dir, "b.log")))

	_, err := newReplay(t, filepath.Join(dir, "*.log"), newMockConsumer()).ReplayCount()
	assert.ErrorContains(t, err, "b.log")
}

// TestRandomMode_GlobSkipsBrokenMatches pins that random mode skips, with a
// warning each, a glob match it cannot stat and a matched directory it cannot
// list, and keeps the readable files.
func TestRandomMode_GlobSkipsBrokenMatches(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root lists directories regardless of permissions")
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "a"), 0o700))
	good := filepath.Join(root, "a", "1.log")
	writeFile(t, good, "l1\n")
	locked := filepath.Join(root, "b")
	require.NoError(t, os.Mkdir(locked, 0o700))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	require.NoError(t, os.Symlink(filepath.Join(root, "gone.log"), filepath.Join(root, "c.log")))

	core, logs := observer.New(zap.WarnLevel)
	gen, err := New(zap.New(core), 1, time.Millisecond, filepath.Join(root, "*"), true, 0, newMockConsumer(), nil, embed.NopTelemetry())
	require.NoError(t, err)
	files, err := gen.getFilesFromAutoDetect()
	require.NoError(t, err)
	assert.Equal(t, []string{good}, files)
	assert.Equal(t, 2, logs.FilterMessageSnippet("Skipping a matched").Len())
}
