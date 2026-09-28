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
			// workers was removed (rate is the load knob); unset is valid.
			name: "workers unset",
			config: HostMetricsGeneratorConfig{
				Rate: time.Second,
			},
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
			name: "valid OS macos",
			config: HostMetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				OS:      "macos",
			},
		},
		{
			name: "valid OS darwin alias",
			config: HostMetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				OS:      "darwin",
			},
		},
		{
			name: "invalid OS",
			config: HostMetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				OS:      "solaris",
			},
			wantErr: true,
			errMsg:  "unsupported OS",
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
