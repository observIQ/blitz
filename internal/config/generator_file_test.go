package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestFileGeneratorConfig_Mode pins the filegen mode values and that replay,
// one ordered stream, requires a single worker.
func TestFileGeneratorConfig_Mode(t *testing.T) {
	base := FileGeneratorConfig{Workers: 1, Rate: time.Second, Source: "capture.log"}

	// file, directory and package are the mode key's old source-type values;
	// a config that still sets one keeps loading and runs random.
	for _, mode := range []string{"", FileModeRandom, FileModeReplay, "file", "directory", "package"} {
		c := base
		c.Mode = mode
		assert.NoError(t, c.Validate(), "mode %q", mode)
	}

	bad := base
	bad.Mode = "sequential"
	assert.ErrorContains(t, bad.Validate(), `mode must be one of random|replay, got "sequential"`)

	multi := base
	multi.Mode = FileModeReplay
	multi.Workers = 2
	assert.ErrorContains(t, multi.Validate(), "replay mode requires exactly 1 worker")

	multi.Mode = FileModeRandom
	assert.NoError(t, multi.Validate(), "random mode allows several workers")
}

// TestReplayMustBeTheOnlyGenerator pins that a filegen replay cannot share a
// generators list, since its end-of-input finish needs a count of its own.
func TestReplayMustBeTheOnlyGenerator(t *testing.T) {
	replay := Generator{Type: GeneratorTypeFile, Filegen: FileGeneratorConfig{Workers: 1, Rate: time.Second, Source: "capture.log", Mode: FileModeReplay}}
	json := Generator{Type: GeneratorTypeJSON, JSON: JSONGeneratorConfig{Workers: 1, Rate: time.Second, Type: "default"}}
	valid := func(gens ...Generator) error {
		c := &Config{Logging: Logging{Type: "stdout", Level: "info"}, Generators: gens, Output: Output{Type: OutputTypeNop}, Metrics: Metrics{Port: 9100}, OnFinish: "exit"}
		return c.Validate()
	}

	assert.NoError(t, valid(replay), "a single-entry list is fine")
	assert.ErrorContains(t, valid(replay, json), "filegen replay must be the only generator")
	assert.ErrorContains(t, valid(json, replay), "filegen replay must be the only generator")
	assert.ErrorContains(t, valid(replay, replay), "filegen replay must be the only generator")
}
