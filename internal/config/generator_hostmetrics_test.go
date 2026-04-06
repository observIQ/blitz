package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostMetricsGeneratorConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  HostMetricsGeneratorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: HostMetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
			},
		},
		{
			name: "valid with OS",
			config: HostMetricsGeneratorConfig{
				Workers: 2,
				Rate:    time.Second,
				OS:      "linux",
			},
		},
		{
			name: "valid with scrapers",
			config: HostMetricsGeneratorConfig{
				Workers:  1,
				Rate:     time.Second,
				Scrapers: []string{"cpu", "memory"},
			},
		},
		{
			name: "invalid workers",
			config: HostMetricsGeneratorConfig{
				Workers: 0,
				Rate:    time.Second,
			},
			wantErr: true,
			errMsg:  "workers must be 1 or greater",
		},
		{
			name: "invalid rate",
			config: HostMetricsGeneratorConfig{
				Workers: 1,
				Rate:    0,
			},
			wantErr: true,
			errMsg:  "rate must be positive",
		},
		{
			name: "invalid OS",
			config: HostMetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				OS:      "macos",
			},
			wantErr: true,
			errMsg:  "OS must be one of",
		},
		{
			name: "invalid scraper",
			config: HostMetricsGeneratorConfig{
				Workers:  1,
				Rate:     time.Second,
				Scrapers: []string{"cpu", "bogus"},
			},
			wantErr: true,
			errMsg:  "invalid scraper",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
