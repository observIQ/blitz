package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracesGeneratorConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  TracesGeneratorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: TracesGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
			},
		},
		{
			name: "invalid workers",
			config: TracesGeneratorConfig{
				Workers: 0,
				Rate:    time.Second,
			},
			wantErr: true,
			errMsg:  "workers must be 1 or greater",
		},
		{
			name: "invalid rate",
			config: TracesGeneratorConfig{
				Workers: 1,
				Rate:    0,
			},
			wantErr: true,
			errMsg:  "rate must be positive",
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
