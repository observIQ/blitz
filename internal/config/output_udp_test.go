package config

import (
	"strings"
	"testing"
)

func TestUDPOutputConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  UDPOutputConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: UDPOutputConfig{
				Host:    "localhost",
				Port:    8080,
				Workers: 2,
			},
			wantErr: false,
		},
		{
			name: "valid configuration with IP address",
			config: UDPOutputConfig{
				Host:    "127.0.0.1",
				Port:    8080,
				Workers: 1,
			},
			wantErr: false,
		},
		{
			name: "valid configuration with zero workers",
			config: UDPOutputConfig{
				Host:    "localhost",
				Port:    8080,
				Workers: 0,
			},
			wantErr: false,
		},
		{
			name: "empty host",
			config: UDPOutputConfig{
				Host:    "",
				Port:    8080,
				Workers: 1,
			},
			wantErr: true,
			errMsg:  "UDP output host validation failed",
		},
		{
			name: "invalid port - zero",
			config: UDPOutputConfig{
				Host:    "localhost",
				Port:    0,
				Workers: 1,
			},
			wantErr: true,
			errMsg:  "UDP output port validation failed",
		},
		{
			name: "invalid port - too high",
			config: UDPOutputConfig{
				Host:    "localhost",
				Port:    65536,
				Workers: 1,
			},
			wantErr: true,
			errMsg:  "UDP output port validation failed",
		},
		{
			name: "negative workers",
			config: UDPOutputConfig{
				Host:    "localhost",
				Port:    8080,
				Workers: -1,
			},
			wantErr: true,
			errMsg:  "UDP output workers cannot be negative, got -1",
		},
		{
			name: "invalid hostname",
			config: UDPOutputConfig{
				Host:    "invalid..hostname",
				Port:    8080,
				Workers: 1,
			},
			wantErr: true,
			errMsg:  "UDP output host validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("UDPOutputConfig.Validate() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("UDPOutputConfig.Validate() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("UDPOutputConfig.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}
