package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHECOutputConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  HECOutputConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: HECOutputConfig{
				Host:  "localhost",
				Port:  8088,
				Token: "test-token",
			},
		},
		{
			name: "missing host",
			config: HECOutputConfig{
				Port:  8088,
				Token: "test-token",
			},
			wantErr: true,
			errMsg:  "host",
		},
		{
			name: "missing port",
			config: HECOutputConfig{
				Host:  "localhost",
				Token: "test-token",
			},
			wantErr: true,
			errMsg:  "port",
		},
		{
			name: "missing token",
			config: HECOutputConfig{
				Host: "localhost",
				Port: 8088,
			},
			wantErr: true,
			errMsg:  "token",
		},
		{
			name: "negative workers",
			config: HECOutputConfig{
				Host:    "localhost",
				Port:    8088,
				Token:   "test-token",
				Workers: -1,
			},
			wantErr: true,
			errMsg:  "workers",
		},
		{
			name: "negative batch size",
			config: HECOutputConfig{
				Host:      "localhost",
				Port:      8088,
				Token:     "test-token",
				BatchSize: -1,
			},
			wantErr: true,
			errMsg:  "batch size",
		},
		{
			name: "negative max retries",
			config: HECOutputConfig{
				Host:       "localhost",
				Port:       8088,
				Token:      "test-token",
				MaxRetries: -1,
			},
			wantErr: true,
			errMsg:  "max retries",
		},
		{
			name: "invalid event format",
			config: HECOutputConfig{
				Host:        "localhost",
				Port:        8088,
				Token:       "test-token",
				EventFormat: "invalid",
			},
			wantErr: true,
			errMsg:  "event format",
		},
		{
			name: "valid raw event format",
			config: HECOutputConfig{
				Host:        "localhost",
				Port:        8088,
				Token:       "test-token",
				EventFormat: HECEventFormatRaw,
			},
		},
		{
			name: "valid parsed event format",
			config: HECOutputConfig{
				Host:        "localhost",
				Port:        8088,
				Token:       "test-token",
				EventFormat: HECEventFormatParsed,
			},
		},
		{
			name: "empty event format defaults ok",
			config: HECOutputConfig{
				Host:  "localhost",
				Port:  8088,
				Token: "test-token",
			},
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
