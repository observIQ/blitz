package filegen

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// Packages lists the union of disk and embedded packages, marking which
// layers hold each and flagging disk-over-embedded overrides (PIPE-1454).
func TestLibrary_Packages_MarksOverrides(t *testing.T) {
	disk := fstest.MapFS{"apache/access.log": {Data: []byte("d")}, "nginx/access.log": {Data: []byte("d")}}
	embedded := fstest.MapFS{"apache/access.log": {Data: []byte("e")}, "cisco/access.log": {Data: []byte("e")}}
	l := Library{disk: disk, embedded: embedded, diskPath: "/x"}

	pkgs, err := l.Packages()
	require.NoError(t, err)

	var names []string
	got := map[string]PackageInfo{}
	for _, p := range pkgs {
		names = append(names, p.Name)
		got[p.Name] = p
	}
	require.Equal(t, []string{"apache", "cisco", "nginx"}, names)
	require.True(t, got["apache"].Overrides())
	require.True(t, got["nginx"].OnDisk && !got["nginx"].OnEmbedded)
	require.True(t, got["cisco"].OnEmbedded && !got["cisco"].OnDisk)
}

// ActiveSource reports where the library resolves from, flagging when disk
// overrides an embedded copy (PIPE-1454).
func TestLibrary_ActiveSource(t *testing.T) {
	disk := fstest.MapFS{"apache/access.log": {Data: []byte("d")}}
	withContent := fstest.MapFS{"apache/access.log": {Data: []byte("e")}}
	empty := fstest.MapFS{}

	require.Equal(t, "/x (overriding embedded)", Library{disk: disk, embedded: withContent, diskPath: "/x"}.ActiveSource())
	require.Equal(t, "/x", Library{disk: disk, embedded: empty, diskPath: "/x"}.ActiveSource())
	require.Equal(t, "embedded", Library{disk: nil, embedded: withContent}.ActiveSource())
	require.Equal(t, "", Library{disk: nil, embedded: empty}.ActiveSource())
}

func libFixture() Library {
	disk := fstest.MapFS{
		"apache/access.log": {Data: []byte("disk-apache")},
		"apache/error.log":  {Data: []byte("disk-apache-err")},
		"nginx/access.log":  {Data: []byte("disk-nginx")},
	}
	embedded := fstest.MapFS{
		"apache/access.log": {Data: []byte("embed-apache")},
		"apache/audit.log":  {Data: []byte("embed-audit")},
		"cisco/asa.log":     {Data: []byte("embed-cisco")},
	}
	return Library{disk: disk, embedded: embedded, diskPath: "/x"}
}

func TestLibrary_Search(t *testing.T) {
	pkgs, err := libFixture().Search("ap")
	require.NoError(t, err)
	var names []string
	for _, p := range pkgs {
		names = append(names, p.Name)
	}
	require.Equal(t, []string{"apache"}, names)
}

func TestLibrary_Files_MarksOverrides(t *testing.T) {
	files, err := libFixture().Files("apache")
	require.NoError(t, err)
	got := map[string]FileInfo{}
	var names []string
	for _, f := range files {
		got[f.Name] = f
		names = append(names, f.Name)
	}
	require.Equal(t, []string{"access.log", "audit.log", "error.log"}, names)
	require.True(t, got["access.log"].Overrides())
	require.True(t, got["error.log"].OnDisk && !got["error.log"].OnEmbedded)
	require.True(t, got["audit.log"].OnEmbedded && !got["audit.log"].OnDisk)
}

func TestLibrary_Show_UsesOverlay(t *testing.T) {
	out, err := libFixture().Show("apache")
	require.NoError(t, err)
	require.Contains(t, out, "disk-apache")
	require.Contains(t, out, "embed-audit")
	require.NotContains(t, out, "embed-apache")
}

func TestLibrary_Diff(t *testing.T) {
	entries, err := libFixture().Diff()
	require.NoError(t, err)
	got := map[string]string{}
	for _, e := range entries {
		got[e.Path] = e.Status
	}
	require.Equal(t, "override", got["apache/access.log"])
	require.Equal(t, "added", got["apache/error.log"])
	require.Equal(t, "added", got["nginx/access.log"])
	_, ok := got["apache/audit.log"]
	require.False(t, ok)
	_, ok = got["cisco/asa.log"]
	require.False(t, ok)
}

func TestLibrary_Extract(t *testing.T) {
	dest := t.TempDir()
	require.NoError(t, libFixture().Extract("apache", dest))
	b, err := os.ReadFile(filepath.Join(dest, "data_library", "apache", "access.log"))
	require.NoError(t, err)
	require.Equal(t, "disk-apache", string(b))
	_, err = os.Stat(filepath.Join(dest, "data_library", "apache", "audit.log"))
	require.NoError(t, err)
}

func TestLibrary_ExtractAll(t *testing.T) {
	dest := t.TempDir()
	require.NoError(t, libFixture().ExtractAll(dest))
	for _, p := range []string{"apache", "cisco", "nginx"} {
		_, err := os.Stat(filepath.Join(dest, "data_library", p))
		require.NoError(t, err)
	}
}
