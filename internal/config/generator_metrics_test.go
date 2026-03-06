package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetricDefinition_Validate(t *testing.T) {
	tests := []struct {
		name    string
		def     MetricDefinition
		wantErr bool
	}{
		{
			name: "valid gauge",
			def: MetricDefinition{
				Name:     "system.cpu.utilization",
				Type:     "gauge",
				ValueMin: 0,
				ValueMax: 100,
			},
		},
		{
			name: "valid sum",
			def: MetricDefinition{
				Name:     "http.requests",
				Type:     "sum",
				ValueMin: 1,
				ValueMax: 50,
			},
		},
		{
			name:    "empty name",
			def:     MetricDefinition{Type: "gauge"},
			wantErr: true,
		},
		{
			name: "valid counter",
			def: MetricDefinition{
				Name:     "http.requests",
				Type:     "counter",
				ValueMin: 1,
				ValueMax: 50,
			},
		},
		{
			name: "valid histogram",
			def: MetricDefinition{
				Name:     "http.duration",
				Type:     "histogram",
				ValueMin: 0,
				ValueMax: 5,
			},
		},
		{
			name: "invalid type",
			def: MetricDefinition{
				Name: "m",
				Type: "exponential_histogram",
			},
			wantErr: true,
		},
		{
			name: "valueMax less than valueMin",
			def: MetricDefinition{
				Name:     "m",
				Type:     "gauge",
				ValueMin: 100,
				ValueMax: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.def.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricsGeneratorConfig_Validate(t *testing.T) {
	validMetric := MetricDefinition{
		Name:     "cpu",
		Type:     "gauge",
		ValueMin: 0,
		ValueMax: 1,
	}

	tests := []struct {
		name    string
		cfg     MetricsGeneratorConfig
		wantErr bool
	}{
		{
			name: "valid",
			cfg: MetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				Metrics: []MetricDefinition{validMetric},
			},
		},
		{
			name: "valid with resource attributes",
			cfg: MetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				ResourceAttributes: map[string][]string{
					"service.name": {"svc-a", "svc-b"},
				},
				Metrics: []MetricDefinition{validMetric},
			},
		},
		{
			name: "zero workers",
			cfg: MetricsGeneratorConfig{
				Workers: 0,
				Rate:    time.Second,
				Metrics: []MetricDefinition{validMetric},
			},
			wantErr: true,
		},
		{
			name: "zero rate",
			cfg: MetricsGeneratorConfig{
				Workers: 1,
				Rate:    0,
				Metrics: []MetricDefinition{validMetric},
			},
			wantErr: true,
		},
		{
			name: "no metrics",
			cfg: MetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				Metrics: nil,
			},
			wantErr: true,
		},
		{
			name: "invalid metric definition",
			cfg: MetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				Metrics: []MetricDefinition{{Name: "", Type: "gauge"}},
			},
			wantErr: true,
		},
		{
			name: "empty resource attribute values",
			cfg: MetricsGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
				ResourceAttributes: map[string][]string{
					"service.name": {},
				},
				Metrics: []MetricDefinition{validMetric},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
