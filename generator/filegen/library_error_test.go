package filegen

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// No library anywhere yields the explicit "data library not found" error,
// not a package-name error (PIPE-1445).
func TestGetFiles_MissingLibrary_ExplicitError(t *testing.T) {
	t.Setenv("BLITZ_DATA_LIBRARY_DIR", "")
	g := &FileLogGenerator{logger: zap.NewNop(), source: "package:apache"}

	_, err := g.getFiles()
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "data library not found")
	require.Contains(t, msg, "/usr/share/blitz/data_library")
	require.Contains(t, msg, "BLITZ_DATA_LIBRARY_DIR")
	require.NotContains(t, msg, "not found in the data library")
}

// A present library missing the package names the package, not the library.
func TestGetFiles_MissingPackage_NamesPackage(t *testing.T) {
	t.Setenv("BLITZ_DATA_LIBRARY_DIR", "")
	base := fstest.MapFS{"nginx/access.log": {Data: []byte("x")}}
	g := &FileLogGenerator{logger: zap.NewNop(), source: "package:apache", dataLibrary: base}

	_, err := g.getFiles()
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "apache")
	require.Contains(t, msg, "not found in the data library")
	require.NotContains(t, msg, "data library not found")
}
