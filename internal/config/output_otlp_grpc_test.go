package config

import (
	"strings"
	"testing"
	"time"
)

func TestOTLPGrpcOutputConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  OTLPGrpcOutputConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: OTLPGrpcOutputConfig{
				Host:               "localhost",
				Port:               4317,
				Workers:            2,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
			wantErr: false,
		},
		{
			name: "valid configuration with IP address",
			config: OTLPGrpcOutputConfig{
				Host:               "127.0.0.1",
				Port:               4317,
				Workers:            1,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
			wantErr: false,
		},
		{
			name: "valid configuration with zero workers",
			config: OTLPGrpcOutputConfig{
				Host:               "localhost",
				Port:               4317,
				Workers:            0,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
			wantErr: false,
		},
		{
			name: "empty host",
			config: OTLPGrpcOutputConfig{
				Host:               "",
				Port:               4317,
				Workers:            1,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
			wantErr: true,
			errMsg:  "OTLP gRPC output host validation failed",
		},
		{
			name: "invalid port - zero",
			config: OTLPGrpcOutputConfig{
				Host:               "localhost",
				Port:               0,
				Workers:            1,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
			wantErr: true,
			errMsg:  "OTLP gRPC output port validation failed",
		},
		{
			name: "invalid port - too high",
			config: OTLPGrpcOutputConfig{
				Host:               "localhost",
				Port:               65536,
				Workers:            1,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
			wantErr: true,
			errMsg:  "OTLP gRPC output port validation failed",
		},
		{
			name: "negative workers",
			config: OTLPGrpcOutputConfig{
				Host:               "localhost",
				Port:               4317,
				Workers:            -1,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
			wantErr: true,
			errMsg:  "OTLP gRPC output workers cannot be negative, got -1",
		},
		{
			name: "negative max queue size",
			config: OTLPGrpcOutputConfig{
				Host:               "localhost",
				Port:               4317,
				Workers:            1,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       -1,
				MaxExportBatchSize: 512,
			},
			wantErr: true,
			errMsg:  "OTLP gRPC output max queue size cannot be negative, got -1",
		},
		{
			name: "negative max export batch size",
			config: OTLPGrpcOutputConfig{
				Host:               "localhost",
				Port:               4317,
				Workers:            1,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: -1,
			},
			wantErr: true,
			errMsg:  "OTLP gRPC output max export batch size cannot be negative, got -1",
		},
		{
			name: "invalid hostname",
			config: OTLPGrpcOutputConfig{
				Host:               "invalid..hostname",
				Port:               4317,
				Workers:            1,
				BatchTimeout:       5 * time.Second,
				MaxQueueSize:       2048,
				MaxExportBatchSize: 512,
			},
			wantErr: true,
			errMsg:  "OTLP gRPC output host validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("OTLPGrpcOutputConfig.Validate() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("OTLPGrpcOutputConfig.Validate() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("OTLPGrpcOutputConfig.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}
