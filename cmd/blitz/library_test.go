package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func runLibrary(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newLibraryCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	return func() (string, error) { err := cmd.Execute(); return buf.String(), err }()
}

func writeLibFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []struct{ path, data string }{
		{"apache/access.log", "apache-line"},
		{"nginx/access.log", "nginx-line"},
	} {
		full := filepath.Join(dir, filepath.FromSlash(f.path))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(f.data+"\n"), 0o644))
	}
	return dir
}

func TestLibraryCmd(t *testing.T) {
	tmp := writeLibFixture(t)
	t.Setenv("BLITZ_DATA_LIBRARY_DIR", tmp)

	out, err := runLibrary(t, "ls")
	require.NoError(t, err)
	require.Contains(t, out, "apache")
	require.Contains(t, out, "nginx")

	out, err = runLibrary(t, "ls", "apache")
	require.NoError(t, err)
	require.Contains(t, out, "access.log")

	out, err = runLibrary(t, "search", "ngin")
	require.NoError(t, err)
	require.Contains(t, out, "nginx")
	require.NotContains(t, out, "apache")

	out, err = runLibrary(t, "show", "apache")
	require.NoError(t, err)
	require.Contains(t, out, "apache-line")

	out, err = runLibrary(t, "path")
	require.NoError(t, err)
	require.Contains(t, out, tmp)

	// Disk-only (no embedded baseline in a bare test build): nothing to diff.
	out, err = runLibrary(t, "diff")
	require.NoError(t, err)
	require.Contains(t, out, "no differences")

	dest := t.TempDir()
	_, err = runLibrary(t, "extract", "apache", dest)
	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(dest, "data_library", "apache", "access.log"))
	require.NoError(t, statErr)
}
