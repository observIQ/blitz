package config

import (
	"strings"
	"testing"
	"time"
)

func TestOutput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		output  Output
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid TCP output",
			output: Output{
				Type: OutputTypeTCP,
				TCP: TCPOutputConfig{
					Host:    "localhost",
					Port:    8080,
					Workers: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "valid UDP output",
			output: Output{
				Type: OutputTypeUDP,
				UDP: UDPOutputConfig{
					Host:    "127.0.0.1",
					Port:    9090,
					Workers: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "empty output type",
			output: Output{
				Type: "",
				TCP: TCPOutputConfig{
					Host:    "localhost",
					Port:    8080,
					Workers: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid output type",
			output: Output{
				Type: "invalid",
				TCP: TCPOutputConfig{
					Host:    "localhost",
					Port:    8080,
					Workers: 1,
				},
			},
			wantErr: true,
			errMsg:  "invalid output type: invalid, must be one of: nop, stdout, tcp, udp, syslog, otlp-grpc, file",
		},
		{
			name: "TCP output with invalid config",
			output: Output{
				Type: OutputTypeTCP,
				TCP: TCPOutputConfig{
					Host:    "",
					Port:    0,
					Workers: 1,
				},
			},
			wantErr: true,
			errMsg:  "TCP output validation failed",
		},
		{
			name: "UDP output with invalid config",
			output: Output{
				Type: OutputTypeUDP,
				UDP: UDPOutputConfig{
					Host:    "",
					Port:    0,
					Workers: 1,
				},
			},
			wantErr: true,
			errMsg:  "UDP output validation failed",
		},
		{
			name: "valid OTLP gRPC output",
			output: Output{
				Type: OutputTypeOTLPGrpc,
				OTLPGrpc: OTLPGrpcOutputConfig{
					Host:               "localhost",
					Port:               4317,
					Workers:            2,
					BatchTimeout:       5 * time.Second,
					MaxQueueSize:       2048,
					MaxExportBatchSize: 512,
				},
			},
			wantErr: false,
		},
		{
			name: "OTLP gRPC output with invalid config",
			output: Output{
				Type: OutputTypeOTLPGrpc,
				OTLPGrpc: OTLPGrpcOutputConfig{
					Host:    "",
					Port:    0,
					Workers: 1,
				},
			},
			wantErr: true,
			errMsg:  "OTLP gRPC output validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.output.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Output.Validate() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Output.Validate() error = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Output.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestPrometheusRemoteWriteOutputErrorNamesItsType pins that an invalid
// remote-write output's error says which output failed.
func TestPrometheusRemoteWriteOutputErrorNamesItsType(t *testing.T) {
	err := (&Output{Type: OutputTypePrometheusRemoteWrite}).Validate()
	if err == nil || !strings.Contains(err.Error(), "prometheus-remote-write output validation failed") {
		t.Fatalf("Validate() = %v, want a prometheus-remote-write output validation error", err)
	}
}

// TestPrometheusScrapeOutputErrorNamesItsType pins that an invalid scrape
// output's error says which output failed.
func TestPrometheusScrapeOutputErrorNamesItsType(t *testing.T) {
	err := (&Output{Type: OutputTypePrometheusScrape, PrometheusScrape: PrometheusScrapeOutputConfig{MetricExpiration: -1}}).Validate()
	if err == nil || !strings.Contains(err.Error(), "prometheus-scrape output validation failed") {
		t.Fatalf("Validate() = %v, want a prometheus-scrape output validation error", err)
	}
}

// TestFlowOutputErrorNamesItsType pins that an invalid flow output's error
// says which output failed.
func TestFlowOutputErrorNamesItsType(t *testing.T) {
	err := (&Output{Type: OutputTypeFlow}).Validate()
	if err == nil || !strings.Contains(err.Error(), "flow output validation failed") {
		t.Fatalf("Validate() = %v, want a flow output validation error", err)
	}
}

// TestFlowGeneratorErrorNamesItsType pins that an invalid flow generator's
// error says which generator failed.
func TestFlowGeneratorErrorNamesItsType(t *testing.T) {
	err := (&Generator{Type: GeneratorTypeFlow}).Validate()
	if err == nil || !strings.Contains(err.Error(), "flow generator validation failed") {
		t.Fatalf("Validate() = %v, want a flow generator validation error", err)
	}
}
