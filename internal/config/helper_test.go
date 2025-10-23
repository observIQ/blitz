package config

import (
	"strings"
	"testing"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid port",
			port:    8080,
			wantErr: false,
		},
		{
			name:    "valid port minimum",
			port:    1,
			wantErr: false,
		},
		{
			name:    "valid port maximum",
			port:    65535,
			wantErr: false,
		},
		{
			name:    "invalid port - zero",
			port:    0,
			wantErr: true,
			errMsg:  "port must be between 1 and 65535, got 0",
		},
		{
			name:    "invalid port - too high",
			port:    65536,
			wantErr: true,
			errMsg:  "port must be between 1 and 65535, got 65536",
		},
		{
			name:    "negative port",
			port:    -1,
			wantErr: true,
			errMsg:  "port must be between 1 and 65535, got -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePort() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidatePort() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePort() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid IPv4 address",
			host:    "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "valid IPv6 address",
			host:    "::1",
			wantErr: false,
		},
		{
			name:    "valid hostname",
			host:    "localhost",
			wantErr: false,
		},
		{
			name:    "valid domain name",
			host:    "example.com",
			wantErr: false,
		},
		{
			name:    "empty host",
			host:    "",
			wantErr: true,
			errMsg:  "host cannot be empty",
		},
		{
			name:    "invalid hostname - double dots",
			host:    "invalid..hostname",
			wantErr: true,
			errMsg:  "host must be a valid IP address or hostname",
		},
		{
			name:    "invalid hostname - starts with dash",
			host:    "-invalid.hostname",
			wantErr: true,
			errMsg:  "host must be a valid IP address or hostname",
		},
		{
			name:    "invalid hostname - ends with dash",
			host:    "invalid.hostname-",
			wantErr: true,
			errMsg:  "host must be a valid IP address or hostname",
		},
		{
			name:    "invalid hostname - too long",
			host:    "a" + strings.Repeat(".a", 127), // Creates a hostname longer than 253 chars
			wantErr: true,
			errMsg:  "host must be a valid IP address or hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHost(tt.host)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateHost() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateHost() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateHost() unexpected error = %v", err)
				}
			}
		})
	}
}
