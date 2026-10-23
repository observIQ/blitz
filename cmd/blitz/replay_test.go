package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runReplay runs blitz with a filegen replay of input to a file output and
// returns what the output wrote. count 0 leaves the run unbounded by config.
func runReplay(t *testing.T, input string, count int) string {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "capture.log")
	out := filepath.Join(dir, "out.log")
	require.NoError(t, os.WriteFile(in, []byte(input), 0o600))
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	yaml := fmt.Sprintf(`generator:
  type: filegen
  count: %d
  filegen:
    source: %s
    mode: replay
    rate: 1ms
output:
  type: file
  file:
    path: %s
metrics:
  port: %d
`, count, in, out, port)
	path := filepath.Join(dir, "blitz.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	runEnds(t)
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	return string(got)
}

// TestRunFilegenReplayEndsAtEndOfInput pins the replay contract end to end: a
// capture with hard-coded timestamps replays byte for byte, every non-empty
// line once and in order, and the run exits at the end of the input.
func TestRunFilegenReplayEndsAtEndOfInput(t *testing.T) {
	capture := "2026-01-02T03:04:05.123Z host1 sshd[42]: Accepted publickey for alice\n" +
		"\n" +
		"<134>Jan  2 03:04:06 host1 nginx: GET /a?b=1 200 \"Mozilla/5.0\" 100% \t tabs\n" +
		"{\"ts\":1767323047,\"msg\":\"json line\"}\n"
	want := "2026-01-02T03:04:05.123Z host1 sshd[42]: Accepted publickey for alice\n" +
		"<134>Jan  2 03:04:06 host1 nginx: GET /a?b=1 200 \"Mozilla/5.0\" 100% \t tabs\n" +
		"{\"ts\":1767323047,\"msg\":\"json line\"}\n"
	assert.Equal(t, want, runReplay(t, capture, 0))
	assert.Equal(t, want, runReplay(t, capture, 0), "a second run is byte-identical")
}

// TestRunFilegenReplayStopsAtCount pins that generator.count below the line
// count ends the replay early.
func TestRunFilegenReplayStopsAtCount(t *testing.T) {
	assert.Equal(t, "a\nb\n", runReplay(t, "a\nb\nc\n", 2))
}

// TestRunFilegenReplayCountAboveInput pins that a count above the line count
// still ends the run at the end of the input instead of hanging.
func TestRunFilegenReplayCountAboveInput(t *testing.T) {
	assert.Equal(t, "a\nb\n", runReplay(t, "a\nb\n", 10))
}

// TestRunFilegenReplayMissingSource pins that the CLI reports an unresolvable
// replay source instead of starting.
func TestRunFilegenReplayMissingSource(t *testing.T) {
	dir := t.TempDir()
	yaml := fmt.Sprintf(`generator:
  type: filegen
  filegen:
    source: %s
    mode: replay
output:
  type: nop
`, filepath.Join(dir, "missing.log"))
	path := filepath.Join(dir, "blitz.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	assert.ErrorContains(t, run(nil, nil), "resolve replay input")
}

// TestRunFilegenReplayListWithRunCount pins that a single-entry generators
// list with a run-level count above the line count still ends at the end of
// the input, that the run-level count overrides the entry's lower count, and
// that a run-level count below the line count ends the replay early.
func TestRunFilegenReplayListWithRunCount(t *testing.T) {
	for name, tc := range map[string]struct{ entryCount, runCount, want string }{
		"no entry count":    {"", "10", "a\nb\nc\n"},
		"lower entry count": {"\n    count: 2", "10", "a\nb\nc\n"},
		"lower run count":   {"", "2", "a\nb\n"},
	} {
		t.Run(name, func(t *testing.T) { replayListWithRunCount(t, tc.entryCount, tc.runCount, tc.want) })
	}
}

func replayListWithRunCount(t *testing.T, entryCount, runCount, want string) {
	dir := t.TempDir()
	in := filepath.Join(dir, "capture.log")
	out := filepath.Join(dir, "out.log")
	require.NoError(t, os.WriteFile(in, []byte("a\nb\nc\n"), 0o600))
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	yaml := fmt.Sprintf(`generators:
  - type: filegen%s
    filegen:
      source: %s
      mode: replay
      rate: 1ms
output:
  type: file
  file:
    path: %s
metrics:
  port: %d
`, entryCount, in, out, port)
	path := filepath.Join(dir, "blitz.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	origFile := configFile
	t.Cleanup(func() { configFile = origFile })
	configFile = path

	f := cliFlags.Lookup("generator-count")
	require.NoError(t, f.Value.Set(runCount))
	f.Changed = true
	t.Cleanup(func() { _ = f.Value.Set("0"); f.Changed = false })

	runEnds(t)
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

// runEnds runs blitz and fails if the run has not ended within 10s, rather
// than hanging until the go test timeout.
func runEnds(t *testing.T) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run(nil, nil) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("run did not end at the end of the input")
	}
}
