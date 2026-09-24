package catalog

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterAndGet(t *testing.T) {
	ResetForTest()
	p := Product{Name: "ltm", Build: func(_ *rand.Rand, _ *Ctx) string { return "line" }}
	Register(p)

	got, ok := Get("ltm")
	require.True(t, ok)
	require.Equal(t, "ltm", got.Name)
	require.Equal(t, "line", got.Build(rand.New(rand.NewSource(1)), &Ctx{}))
}

func TestRegisterDuplicatePanics(t *testing.T) {
	ResetForTest()
	Register(Product{Name: "ltm", Build: func(_ *rand.Rand, _ *Ctx) string { return "" }})
	require.Panics(t, func() {
		Register(Product{Name: "ltm", Build: func(_ *rand.Rand, _ *Ctx) string { return "" }})
	})
}

func TestAllProductsSortedByName(t *testing.T) {
	ResetForTest()
	Register(Product{Name: "zzz", Build: func(_ *rand.Rand, _ *Ctx) string { return "" }})
	Register(Product{Name: "aaa", Build: func(_ *rand.Rand, _ *Ctx) string { return "" }})

	all := AllProducts()
	require.Len(t, all, 2)
	require.Equal(t, "aaa", all[0].Name)
	require.Equal(t, "zzz", all[1].Name)
}

func TestGetMissing(t *testing.T) {
	ResetForTest()
	_, ok := Get("nope")
	require.False(t, ok)
}
