package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWinevtGeneratorConfig_Validate(t *testing.T) {
	c := WinevtGeneratorConfig{Workers: 1, Rate: time.Second}
	assert.NoError(t, c.Validate())

	c = WinevtGeneratorConfig{Workers: 0, Rate: time.Second}
	assert.Error(t, c.Validate())

	c = WinevtGeneratorConfig{Workers: 1, Rate: 0}
	assert.Error(t, c.Validate())
}
