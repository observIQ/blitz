package config

import "testing"

func TestPrometheusScrapeOutputConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PrometheusScrapeOutputConfig
		wantErr bool
	}{
		{"valid", PrometheusScrapeOutputConfig{ListenAddress: "0.0.0.0:9464", MetricsPath: "/metrics"}, false},
		{"empty uses defaults", PrometheusScrapeOutputConfig{}, false},
		{"emit timestamps ok", PrometheusScrapeOutputConfig{ListenAddress: "127.0.0.1:9464", MetricsPath: "/m", EmitTimestamps: true}, false},
		{"path missing leading slash", PrometheusScrapeOutputConfig{ListenAddress: "0.0.0.0:9464", MetricsPath: "metrics"}, true},
		{"listen address no port", PrometheusScrapeOutputConfig{ListenAddress: "0.0.0.0", MetricsPath: "/metrics"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
