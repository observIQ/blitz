package config

import (
	"testing"
	"time"
)

func TestF5GeneratorConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     F5GeneratorConfig
		wantErr bool
	}{
		{"valid all products", F5GeneratorConfig{Workers: 1, Rate: time.Second}, false},
		{"valid subset", F5GeneratorConfig{Workers: 2, Rate: time.Second, EnabledProducts: []string{"ltm", "asm"}}, false},
		{"workers zero", F5GeneratorConfig{Workers: 0, Rate: time.Second}, true},
		{"rate zero", F5GeneratorConfig{Workers: 1, Rate: 0}, true},
		{"unknown product", F5GeneratorConfig{Workers: 1, Rate: time.Second, EnabledProducts: []string{"nope"}}, true},
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
