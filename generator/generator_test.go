package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/observiq/blitz/internal/datagen"
)

func TestNewOptions(t *testing.T) {
	assert.Nil(t, NewOptions().Identity, "no options leaves the identity unset")

	a := &datagen.SystemIdentity{Hostname: "a"}
	b := &datagen.SystemIdentity{Hostname: "b"}
	assert.Same(t, a, NewOptions(WithIdentity(a)).Identity)
	assert.Same(t, b, NewOptions(WithIdentity(a), WithIdentity(b)).Identity, "the last option wins")
	assert.Nil(t, NewOptions(WithIdentity(nil)).Identity)
}
