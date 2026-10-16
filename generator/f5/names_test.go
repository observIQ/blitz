package f5

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/generator/f5/catalog"
)

// TestProductNamesMatchRegistry pins catalog.Names, which config validation
// reads without importing the generator, to the products this package
// registers.
func TestProductNamesMatchRegistry(t *testing.T) {
	var registered []string
	for _, p := range catalog.AllProducts() {
		registered = append(registered, p.Name)
	}
	require.ElementsMatch(t, registered, catalog.Names)
}
