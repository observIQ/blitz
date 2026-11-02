package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadmeListsEveryType pins that the README lists every generator and
// output type the CLI accepts, each linked to a doc page that exists.
func TestReadmeListsEveryType(t *testing.T) {
	root := filepath.Join("..", "..")
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, err)

	for kind, field := range map[string]string{"generator": "generator.type", "output": "output.type"} {
		types := fieldChoices(t, field)
		require.NotEmpty(t, types, "%s usage lists its types", field)
		for _, typ := range types {
			page := "docs/" + kind + "/" + typ + ".md"
			entry := "**[" + typ + "](https://github.com/observiq/blitz/blob/main/" + page + ")**"
			assert.Contains(t, string(readme), entry, "README lists %s %q linked to %s", kind, typ, page)
			assert.FileExists(t, filepath.Join(root, page))
		}
	}
}
