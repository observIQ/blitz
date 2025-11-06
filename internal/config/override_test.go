package config

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestOverrideDefaults(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	overrides := DefaultOverrides()
	for _, override := range overrides {
		require.NoError(t, override.Bind(flagSet))
	}

	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	cfg := NewConfig()
	err := viper.Unmarshal(cfg)
	require.NoError(t, err)

	// build expected config and compare full struct
	expectedCfg := &Config{
		Logging: Logging{Type: LoggingTypeStdout, Level: LogLevelInfo},
		Generator: Generator{
			JSON: JSONGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			Winevt: WinevtGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
		},
		Output: Output{
			UDP: UDPOutputConfig{Host: "", Port: 0, Workers: 1},
			TCP: TCPOutputConfig{
				Host:      "",
				Port:      0,
				Workers:   1,
				EnableTLS: false,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
				},
			},
			File: FileOutputConfig{
				Path:    "",
				Workers: 1,
				Rotation: FileRotationConfig{
					MaxSizeMB:  DefaultFileRotationMaxSizeMB,
					MaxBackups: DefaultFileRotationMaxBackups,
					MaxAgeDays: DefaultFileRotationMaxAgeDays,
					Compress:   true,
					LocalTime:  false,
				},
			},
			OTLPGrpc: OTLPGrpcOutputConfig{
				Host:               DefaultOTLPGrpcHost,
				Port:               DefaultOTLPGrpcPort,
				Workers:            DefaultOTLPGrpcWorkers,
				BatchTimeout:       DefaultOTLPGrpcBatchTimeout,
				MaxQueueSize:       DefaultOTLPGrpcMaxQueueSize,
				MaxExportBatchSize: DefaultOTLPGrpcMaxExportBatchSize,
				EnableTLS:          false,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
					Insecure:             true,
				},
			},
		},
	}
	require.Equal(t, expectedCfg, cfg)
}

func TestOverrideFlags(t *testing.T) {
	// Ensure environment variables cannot interfere with flag-based expectations
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE", "false")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_SKIP_VERIFY", "false")

	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	args := []string{
		"--logging-type", "stdout",
		"--logging-level", "warn",
		"--generator-type", "json",
		"--generator-json-workers", "5",
		"--generator-json-rate", "500ms",
		"--generator-winevt-workers", "4",
		"--generator-winevt-rate", "200ms",
		"--output-type", "otlp-grpc",

		// UDP options
		"--output-udp-host", "udp.example.com",
		"--output-udp-port", "1514",
		"--output-udp-workers", "2",

		// TCP options (including TLS)
		"--output-tcp-host", "127.0.0.1",
		"--output-tcp-port", "9090",
		"--output-tcp-workers", "3",
		"--output-tcp-enable-tls", "true",
		"--output-tcp-tls-cert", "/path/to/cert.pem",
		"--output-tcp-tls-key", "/path/to/key.pem",
		"--output-tcp-tls-ca", "/path/to/ca1.pem,/path/to/ca2.pem",
		"--output-tcp-tls-skip-verify", "true",
		"--output-tcp-tls-min-version", "1.2",

		// File options
		"--output-file-path", "/tmp/blitz.log",
		"--output-file-workers", "2",
		"--output-file-rotation-maxsizemb", "50",
		"--output-file-rotation-maxbackups", "5",
		"--output-file-rotation-maxagedays", "10",
		"--output-file-rotation-compress", "true",
		"--output-file-rotation-localtime", "true",

		// OTLP gRPC options (including TLS)
		"--output-otlpgrpc-host", "collector.example.com",
		"--output-otlpgrpc-port", "4317",
		"--output-otlpgrpc-workers", "3",
		"--output-otlpgrpc-batchtimeout", "10s",
		"--output-otlpgrpc-maxqueuesize", "4096",
		"--output-otlpgrpc-maxexportbatchsize", "1024",
		"--output-otlpgrpc-enable-tls", "true",
		"--otlp-grpc-tls-cert", "/path/to/otlp_cert.pem",
		"--otlp-grpc-tls-key", "/path/to/otlp_key.pem",
		"--otlp-grpc-tls-ca", "/path/to/otlp_ca1.pem,/path/to/otlp_ca2.pem",

		"--otlp-grpc-tls-min-version", "1.3",
	}

	overrides := DefaultOverrides()
	for _, override := range overrides {
		require.NoError(t, override.Bind(flagSet))
	}

	require.NoError(t, flagSet.Parse(args))

	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	cfg := NewConfig()
	err := viper.Unmarshal(cfg)
	require.NoError(t, err)

	// build expected config and compare full struct
	expectedCfg := &Config{
		Logging: Logging{Type: LoggingTypeStdout, Level: LogLevelWarn},
		Generator: Generator{
			Type: GeneratorTypeJSON,
			JSON: JSONGeneratorConfig{
				Workers: 5,
				Rate:    500 * time.Millisecond,
			},
			Winevt: WinevtGeneratorConfig{
				Workers: 4,
				Rate:    200 * time.Millisecond,
			},
		},
		Output: Output{
			Type: OutputTypeOTLPGrpc,
			UDP:  UDPOutputConfig{Host: "udp.example.com", Port: 1514, Workers: 2},
			TCP: TCPOutputConfig{
				Host:      "127.0.0.1",
				Port:      9090,
				Workers:   3,
				EnableTLS: true,
				TLS: TLS{
					Certificate:          "/path/to/cert.pem",
					PrivateKey:           "/path/to/key.pem",
					CertificateAuthority: []string{"/path/to/ca1.pem", "/path/to/ca2.pem"},
					InsecureSkipVerify:   true,
					MinTLSVersion:        "1.2",
				},
			},
			File: FileOutputConfig{
				Path:    "/tmp/blitz.log",
				Workers: 2,
				Rotation: FileRotationConfig{
					MaxSizeMB:  50,
					MaxBackups: 5,
					MaxAgeDays: 10,
					Compress:   true,
					LocalTime:  true,
				},
			},
			OTLPGrpc: OTLPGrpcOutputConfig{
				Host:               "collector.example.com",
				Port:               4317,
				Workers:            3,
				BatchTimeout:       10 * time.Second,
				MaxQueueSize:       4096,
				MaxExportBatchSize: 1024,
				EnableTLS:          true,
				TLS: TLS{
					Certificate:          "/path/to/otlp_cert.pem",
					PrivateKey:           "/path/to/otlp_key.pem",
					CertificateAuthority: []string{"/path/to/otlp_ca1.pem", "/path/to/otlp_ca2.pem"},
					InsecureSkipVerify:   false,
					Insecure:             false,
					MinTLSVersion:        "1.3",
				},
			},
		},
	}
	require.Equal(t, expectedCfg, cfg)
}

func TestOverrideEnvs(t *testing.T) {
	// Logging
	t.Setenv("BLITZ_LOGGING_TYPE", "stdout")
	t.Setenv("BLITZ_LOGGING_LEVEL", "error")

	// Generators
	t.Setenv("BLITZ_GENERATOR_TYPE", "winevt")
	t.Setenv("BLITZ_GENERATOR_JSON_WORKERS", "3")
	t.Setenv("BLITZ_GENERATOR_JSON_RATE", "250ms")
	t.Setenv("BLITZ_GENERATOR_WINEVT_WORKERS", "2")
	t.Setenv("BLITZ_GENERATOR_WINEVT_RATE", "750ms")

	// Output selection
	t.Setenv("BLITZ_OUTPUT_TYPE", "file")

	// UDP
	t.Setenv("BLITZ_OUTPUT_UDP_HOST", "udp.env.example")
	t.Setenv("BLITZ_OUTPUT_UDP_PORT", "5514")
	t.Setenv("BLITZ_OUTPUT_UDP_WORKERS", "4")

	// TCP + TLS
	t.Setenv("BLITZ_OUTPUT_TCP_HOST", "tcp.env.example")
	t.Setenv("BLITZ_OUTPUT_TCP_PORT", "8081")
	t.Setenv("BLITZ_OUTPUT_TCP_WORKERS", "2")
	t.Setenv("BLITZ_OUTPUT_TCP_ENABLE_TLS", "true")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_CERT", "/env/cert.pem")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_KEY", "/env/key.pem")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_CA", "/env/ca1.pem,/env/ca2.pem")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_SKIP_VERIFY", "true")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_MIN_VERSION", "1.3")

	// File
	t.Setenv("BLITZ_OUTPUT_FILE_PATH", "/env/blitz.log")
	t.Setenv("BLITZ_OUTPUT_FILE_WORKERS", "3")
	t.Setenv("BLITZ_OUTPUT_FILE_ROTATION_MAXSIZEMB", "75")
	t.Setenv("BLITZ_OUTPUT_FILE_ROTATION_MAXBACKUPS", "6")
	t.Setenv("BLITZ_OUTPUT_FILE_ROTATION_MAXAGEDAYS", "20")
	t.Setenv("BLITZ_OUTPUT_FILE_ROTATION_COMPRESS", "false")
	t.Setenv("BLITZ_OUTPUT_FILE_ROTATION_LOCALTIME", "true")

	// OTLP gRPC + TLS
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_HOST", "collector.env.example")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_PORT", "4318")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_WORKERS", "5")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_BATCHTIMEOUT", "15s")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_MAXQUEUESIZE", "8192")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_MAXEXPORTBATCHSIZE", "2048")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_ENABLE_TLS", "true")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE", "false")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_CERT", "/env/otlp_cert.pem")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_KEY", "/env/otlp_key.pem")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_CA", "/env/otlp_ca1.pem,/env/otlp_ca2.pem")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_SKIP_VERIFY", "false")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_MIN_VERSION", "1.2")

	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	overrides := DefaultOverrides()
	for _, override := range overrides {
		require.NoError(t, override.Bind(flagSet))
	}

	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	cfg := NewConfig()
	err := viper.Unmarshal(cfg)
	require.NoError(t, err)

	// build expected config and compare full struct
	expectedCfg := &Config{
		Logging: Logging{Type: LoggingTypeStdout, Level: LogLevelError},
		Generator: Generator{
			Type: GeneratorTypeWinevt,
			JSON: JSONGeneratorConfig{
				Workers: 3,
				Rate:    250 * time.Millisecond,
			},
			Winevt: WinevtGeneratorConfig{
				Workers: 2,
				Rate:    750 * time.Millisecond,
			},
		},
		Output: Output{
			Type: OutputTypeFile,
			UDP:  UDPOutputConfig{Host: "udp.env.example", Port: 5514, Workers: 4},
			TCP: TCPOutputConfig{
				Host:      "tcp.env.example",
				Port:      8081,
				Workers:   2,
				EnableTLS: true,
				TLS: TLS{
					Certificate:          "/env/cert.pem",
					PrivateKey:           "/env/key.pem",
					CertificateAuthority: []string{"/env/ca1.pem", "/env/ca2.pem"},
					InsecureSkipVerify:   true,
					MinTLSVersion:        "1.3",
				},
			},
			File: FileOutputConfig{
				Path:    "/env/blitz.log",
				Workers: 3,
				Rotation: FileRotationConfig{
					MaxSizeMB:  75,
					MaxBackups: 6,
					MaxAgeDays: 20,
					Compress:   false,
					LocalTime:  true,
				},
			},
			OTLPGrpc: OTLPGrpcOutputConfig{
				Host:               "collector.env.example",
				Port:               4318,
				Workers:            5,
				BatchTimeout:       15 * time.Second,
				MaxQueueSize:       8192,
				MaxExportBatchSize: 2048,
				EnableTLS:          true,
				TLS: TLS{
					Certificate:          "/env/otlp_cert.pem",
					PrivateKey:           "/env/otlp_key.pem",
					CertificateAuthority: []string{"/env/otlp_ca1.pem", "/env/otlp_ca2.pem"},
					InsecureSkipVerify:   false,
					Insecure:             false,
					MinTLSVersion:        "1.2",
				},
			},
		},
	}
	require.Equal(t, expectedCfg, cfg)
}
