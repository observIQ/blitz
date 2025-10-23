package config

import (
	"strings"
	"testing"
)

func TestTCPOutputConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  TCPOutputConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: TCPOutputConfig{
				Host:    "localhost",
				Port:    8080,
				Workers: 2,
			},
			wantErr: false,
		},
		{
			name: "valid configuration with IP address",
			config: TCPOutputConfig{
				Host:    "127.0.0.1",
				Port:    8080,
				Workers: 1,
			},
			wantErr: false,
		},
		{
			name: "valid configuration with zero workers",
			config: TCPOutputConfig{
				Host:    "localhost",
				Port:    8080,
				Workers: 0,
			},
			wantErr: false,
		},
		{
			name: "empty host",
			config: TCPOutputConfig{
				Host:    "",
				Port:    8080,
				Workers: 1,
			},
			wantErr: true,
			errMsg:  "TCP output host validation failed",
		},
		{
			name: "invalid port - zero",
			config: TCPOutputConfig{
				Host:    "localhost",
				Port:    0,
				Workers: 1,
			},
			wantErr: true,
			errMsg:  "TCP output port validation failed",
		},
		{
			name: "invalid port - too high",
			config: TCPOutputConfig{
				Host:    "localhost",
				Port:    65536,
				Workers: 1,
			},
			wantErr: true,
			errMsg:  "TCP output port validation failed",
		},
		{
			name: "negative workers",
			config: TCPOutputConfig{
				Host:    "localhost",
				Port:    8080,
				Workers: -1,
			},
			wantErr: true,
			errMsg:  "TCP output workers cannot be negative, got -1",
		},
		{
			name: "invalid hostname",
			config: TCPOutputConfig{
				Host:    "invalid..hostname",
				Port:    8080,
				Workers: 1,
			},
			wantErr: true,
			errMsg:  "TCP output host validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("TCPOutputConfig.Validate() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("TCPOutputConfig.Validate() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("TCPOutputConfig.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}
