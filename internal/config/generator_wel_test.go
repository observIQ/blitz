package config

import (
	"testing"
	"time"
)

func TestWelGeneratorConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  WelGeneratorConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  WelGeneratorConfig{Workers: 2, Rate: time.Second, Role: "member"},
			wantErr: false,
		},
		{
			name:    "valid empty role defaults",
			config:  WelGeneratorConfig{Workers: 1, Rate: time.Second, Role: ""},
			wantErr: false,
		},
		{
			name:    "valid dc role",
			config:  WelGeneratorConfig{Workers: 1, Rate: time.Second, Role: "dc"},
			wantErr: false,
		},
		{
			name:    "zero workers",
			config:  WelGeneratorConfig{Workers: 0, Rate: time.Second},
			wantErr: true,
		},
		{
			name:    "zero rate",
			config:  WelGeneratorConfig{Workers: 1, Rate: 0},
			wantErr: true,
		},
		{
			name:    "invalid role",
			config:  WelGeneratorConfig{Workers: 1, Rate: time.Second, Role: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
