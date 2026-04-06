package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeValid(t *testing.T) {
	tests := []struct {
		name  string
		t     Type
		valid bool
	}{
		{"logs", Logs, true},
		{"metrics", Metrics, true},
		{"traces", Traces, true},
		{"empty", Type(""), false},
		{"unknown", Type("unknown"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.valid, tc.t.Valid())
		})
	}
}

func TestSupports(t *testing.T) {
	tests := []struct {
		name      string
		supported []Type
		target    Type
		expected  bool
	}{
		{
			name:      "logs in logs",
			supported: []Type{Logs},
			target:    Logs,
			expected:  true,
		},
		{
			name:      "metrics in logs+metrics",
			supported: []Type{Logs, Metrics},
			target:    Metrics,
			expected:  true,
		},
		{
			name:      "traces not in logs+metrics",
			supported: []Type{Logs, Metrics},
			target:    Traces,
			expected:  false,
		},
		{
			name:      "traces in all three",
			supported: []Type{Logs, Metrics, Traces},
			target:    Traces,
			expected:  true,
		},
		{
			name:      "empty supported",
			supported: []Type{},
			target:    Logs,
			expected:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, Supports(tc.supported, tc.target))
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		generator []Type
		output    []Type
		wantErr   bool
	}{
		{
			name:      "both support logs",
			generator: []Type{Logs},
			output:    []Type{Logs},
			wantErr:   false,
		},
		{
			name:      "overlap on metrics only",
			generator: []Type{Metrics},
			output:    []Type{Logs, Metrics},
			wantErr:   false,
		},
		{
			name:      "multi overlap is valid",
			generator: []Type{Logs, Metrics, Traces},
			output:    []Type{Logs, Metrics, Traces},
			wantErr:   false,
		},
		{
			name:      "partial overlap is valid",
			generator: []Type{Logs, Metrics},
			output:    []Type{Metrics, Traces},
			wantErr:   false,
		},
		{
			name:      "no overlap is error",
			generator: []Type{Logs},
			output:    []Type{Metrics},
			wantErr:   true,
		},
		{
			name:      "empty generator is error",
			generator: []Type{},
			output:    []Type{Logs},
			wantErr:   true,
		},
		{
			name:      "empty output is error",
			generator: []Type{Logs},
			output:    []Type{},
			wantErr:   true,
		},
		{
			name:      "both empty is error",
			generator: []Type{},
			output:    []Type{},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.generator, tc.output)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
