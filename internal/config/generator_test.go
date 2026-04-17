package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerator_Validate(t *testing.T) {
	tests := []struct {
		name    string
		gen     Generator
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid JSON generator",
			gen: Generator{
				Type: GeneratorTypeJSON,
				JSON: JSONGeneratorConfig{
					Workers: 5,
					Rate:    100 * time.Millisecond,
				},
			},
			wantErr: false,
		},
		{
			name: "empty generator type",
			gen: Generator{
				Type: "",
				JSON: JSONGeneratorConfig{
					Workers: 5,
					Rate:    100 * time.Millisecond,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid generator type",
			gen: Generator{
				Type: "invalid",
				JSON: JSONGeneratorConfig{
					Workers: 5,
					Rate:    100 * time.Millisecond,
				},
			},
			wantErr: true,
			errMsg:  "invalid generator type: invalid, must be one of: nop, json",
		},
		{
			name: "JSON generator validation error",
			gen: Generator{
				Type: GeneratorTypeJSON,
				JSON: JSONGeneratorConfig{
					Workers: 0, // Invalid workers
					Rate:    100 * time.Millisecond,
				},
			},
			wantErr: true,
			errMsg:  "JSON generator validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.gen.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJSONGeneratorConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  JSONGeneratorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: JSONGeneratorConfig{
				Workers: 5,
				Rate:    100 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "zero workers",
			config: JSONGeneratorConfig{
				Workers: 0,
				Rate:    100 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "JSON generator workers must be 1 or greater, got 0",
		},
		{
			name: "negative workers",
			config: JSONGeneratorConfig{
				Workers: -1,
				Rate:    100 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "JSON generator workers must be 1 or greater, got -1",
		},
		{
			name: "zero rate",
			config: JSONGeneratorConfig{
				Workers: 5,
				Rate:    0,
			},
			wantErr: true,
			errMsg:  "JSON generator rate must be positive, got 0s",
		},
		{
			name: "negative rate",
			config: JSONGeneratorConfig{
				Workers: 5,
				Rate:    -100 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "JSON generator rate must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerator_Validate_Count(t *testing.T) {
	tests := []struct {
		name    string
		gen     Generator
		wantErr bool
		errMsg  string
	}{
		{
			name: "zero count is valid (unlimited)",
			gen: Generator{
				Type:  GeneratorTypeJSON,
				Count: 0,
				JSON:  JSONGeneratorConfig{Workers: 1, Rate: 1 * time.Second},
			},
			wantErr: false,
		},
		{
			name: "positive count is valid",
			gen: Generator{
				Type:  GeneratorTypeJSON,
				Count: 100,
				JSON:  JSONGeneratorConfig{Workers: 1, Rate: 1 * time.Second},
			},
			wantErr: false,
		},
		{
			name: "negative count is invalid",
			gen: Generator{
				Type:  GeneratorTypeJSON,
				Count: -1,
				JSON:  JSONGeneratorConfig{Workers: 1, Rate: 1 * time.Second},
			},
			wantErr: true,
			errMsg:  "generator count must be 0 or greater",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.gen.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_OnFinish(t *testing.T) {
	tests := []struct {
		name     string
		onFinish string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "empty is valid (defaults to exit)",
			onFinish: "",
			wantErr:  false,
		},
		{
			name:     "exit is valid",
			onFinish: "exit",
			wantErr:  false,
		},
		{
			name:     "idle is valid",
			onFinish: "idle",
			wantErr:  false,
		},
		{
			name:     "invalid value",
			onFinish: "restart",
			wantErr:  true,
			errMsg:   "onFinish must be one of: exit, idle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Logging:  Logging{Type: LoggingTypeStdout, Level: LogLevelInfo},
				Metrics:  Metrics{Port: DefaultMetricsPort},
				OnFinish: tt.onFinish,
			}
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGeneratorType_Constants(t *testing.T) {
	assert.Equal(t, GeneratorType("json"), GeneratorTypeJSON)
}
