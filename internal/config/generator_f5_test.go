package config

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/generator/f5"
)

func TestF5GeneratorConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     F5GeneratorConfig
		wantErr bool
	}{
		{"valid all products", F5GeneratorConfig{Workers: 1, Rate: time.Second}, false},
		{"valid subset", F5GeneratorConfig{Workers: 2, Rate: time.Second, EnabledProducts: []string{"afm", "asm"}}, false},
		{"workers zero", F5GeneratorConfig{Workers: 0, Rate: time.Second}, true},
		{"rate zero", F5GeneratorConfig{Workers: 1, Rate: 0}, true},
		{"unknown product", F5GeneratorConfig{Workers: 1, Rate: time.Second, EnabledProducts: []string{"nope"}}, true},
		{"duplicate product", F5GeneratorConfig{Workers: 1, Rate: time.Second, EnabledProducts: []string{"afm", "afm"}}, true},
		{"positive weight", F5GeneratorConfig{Workers: 1, Rate: time.Second, Weights: map[string]float64{"asm": 2.5}}, false},
		{"zero weight", F5GeneratorConfig{Workers: 1, Rate: time.Second, Weights: map[string]float64{"asm": 0}}, true},
		{"negative weight", F5GeneratorConfig{Workers: 1, Rate: time.Second, Weights: map[string]float64{"asm": -1}}, true},
		{"unknown weight", F5GeneratorConfig{Workers: 1, Rate: time.Second, Weights: map[string]float64{"nope": 1}}, true},
		{"NaN weight", F5GeneratorConfig{Workers: 1, Rate: time.Second, Weights: map[string]float64{"asm": math.NaN()}}, true},
		{"weights summing past float64", F5GeneratorConfig{Workers: 1, Rate: time.Second, Weights: map[string]float64{"afm": 1e308, "asm": 1e308}}, true},
		{"infinite weight", F5GeneratorConfig{Workers: 1, Rate: time.Second, Weights: map[string]float64{"asm": math.Inf(1)}}, true},
		{"weight for a disabled product", F5GeneratorConfig{Workers: 1, Rate: time.Second, EnabledProducts: []string{"asm"}, Weights: map[string]float64{"afm": 5}}, true},
		{"weight for an enabled product", F5GeneratorConfig{Workers: 1, Rate: time.Second, EnabledProducts: []string{"asm", "afm"}, Weights: map[string]float64{"afm": 5}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

// TestF5OverrideDefaultsMatchGenerator pins the generator.f5.* override
// defaults to f5.DefaultConfig(), so the CLI/env path and the generator
// package cannot drift apart.
func TestF5OverrideDefaultsMatchGenerator(t *testing.T) {
	want := f5.DefaultConfig()
	got := map[string]any{}
	for _, o := range DefaultOverrides() {
		if strings.HasPrefix(o.Field, "generator.f5.") {
			got[strings.TrimPrefix(o.Field, "generator.f5.")] = o.Default
		}
	}
	require.Equal(t, map[string]any{
		"workers":         want.Workers,
		"rate":            want.Rate,
		"hostname":        want.Hostname,
		"seed":            want.Seed,
		"enabledProducts": []string{},
	}, got)
}

// TestF5WeightErrorsAreDeterministic pins that with several bad weights the
// config error always names the same product (the first by name), as the
// generator's own check does.
func TestF5WeightErrorsAreDeterministic(t *testing.T) {
	c := F5GeneratorConfig{Workers: 1, Rate: time.Second, Weights: map[string]float64{"zz-unknown": 1, "mm-unknown": 1, "aa-unknown": 1}}
	for range 50 {
		require.ErrorContains(t, c.Validate(), `"aa-unknown"`)
	}
}
