package catalog

import (
	"math/rand"
	"testing"
)

func TestLiteralField(t *testing.T) {
	g := LiteralField(35, "D")
	got := g(rand.New(rand.NewSource(1)), &GenerateCtx{})
	if got.Tag != 35 || got.Value != "D" {
		t.Errorf("LiteralField result = %+v, want Tag=35 Value=D", got)
	}
}

func TestLiteralFieldIgnoresRNG(t *testing.T) {
	g := LiteralField(35, "D")
	// Calling with a different RNG must give the same result — that's
	// the whole point of a literal.
	a := g(rand.New(rand.NewSource(1)), &GenerateCtx{})
	b := g(rand.New(rand.NewSource(99)), &GenerateCtx{})
	if a != b {
		t.Errorf("LiteralField varies with RNG: %v vs %v", a, b)
	}
}

func TestIntField(t *testing.T) {
	g := IntField(34, 42)
	got := g(rand.New(rand.NewSource(1)), &GenerateCtx{})
	if got.Tag != 34 || got.Value != "42" {
		t.Errorf("IntField(34, 42) = %+v, want Tag=34 Value=42", got)
	}
}
