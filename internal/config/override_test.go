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
		},
		Output: Output{
			UDP: UDPOutputConfig{Workers: 1},
			TCP: TCPOutputConfig{
				Workers:   1,
				EnableTLS: false,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
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
	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	args := []string{
		"--logging-type", "stdout",
		"--logging-level", "warn",
		"--generator-type", "json",
		"--generator-json-workers", "5",
		"--generator-json-rate", "500ms",
		"--output-type", "tcp",
		"--output-tcp-host", "127.0.0.1",
		"--output-tcp-port", "9090",
		"--output-tcp-workers", "3",
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
		},
		Output: Output{
			Type: OutputTypeTCP,
			UDP:  UDPOutputConfig{Workers: 1},
			TCP: TCPOutputConfig{
				Host:      "127.0.0.1",
				Port:      9090,
				Workers:   3,
				EnableTLS: false,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
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

func TestOverrideEnvs(t *testing.T) {
	t.Setenv("BLITZ_LOGGING_TYPE", "stdout")
	t.Setenv("BLITZ_LOGGING_LEVEL", "error")
	t.Setenv("BLITZ_GENERATOR_TYPE", "json")
	t.Setenv("BLITZ_GENERATOR_JSON_WORKERS", "3")
	t.Setenv("BLITZ_GENERATOR_JSON_RATE", "250ms")
	t.Setenv("BLITZ_OUTPUT_TYPE", "udp")
	t.Setenv("BLITZ_OUTPUT_UDP_HOST", "example.com")
	t.Setenv("BLITZ_OUTPUT_UDP_PORT", "8080")
	t.Setenv("BLITZ_OUTPUT_UDP_WORKERS", "4")

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
			Type: GeneratorTypeJSON,
			JSON: JSONGeneratorConfig{
				Workers: 3,
				Rate:    250 * time.Millisecond,
			},
		},
		Output: Output{
			Type: OutputTypeUDP,
			UDP: UDPOutputConfig{
				Host:    "example.com",
				Port:    8080,
				Workers: 4,
			},
			TCP: TCPOutputConfig{
				Workers:   1,
				EnableTLS: false,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
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

func TestOverrideOTLPGrpcFlags(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	args := []string{
		"--output-type", "otlp-grpc",
		"--output-otlpgrpc-host", "collector.example.com",
		"--output-otlpgrpc-port", "4317",
		"--output-otlpgrpc-workers", "3",
		"--output-otlpgrpc-batchtimeout", "10s",
		"--output-otlpgrpc-maxqueuesize", "4096",
		"--output-otlpgrpc-maxexportbatchsize", "1024",
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
		Logging: Logging{Type: LoggingTypeStdout, Level: LogLevelInfo},
		Generator: Generator{
			JSON: JSONGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
		},
		Output: Output{
			Type: OutputTypeOTLPGrpc,
			UDP:  UDPOutputConfig{Workers: 1},
			TCP: TCPOutputConfig{
				Workers:   1,
				EnableTLS: false,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
				},
			},
			OTLPGrpc: OTLPGrpcOutputConfig{
				Host:               "collector.example.com",
				Port:               4317,
				Workers:            3,
				BatchTimeout:       10 * time.Second,
				MaxQueueSize:       4096,
				MaxExportBatchSize: 1024,
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

func TestOverrideOTLPGrpcEnvs(t *testing.T) {
	t.Setenv("BLITZ_OUTPUT_TYPE", "otlp-grpc")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_HOST", "collector.example.com")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_PORT", "4317")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_WORKERS", "5")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_BATCHTIMEOUT", "15s")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_MAXQUEUESIZE", "8192")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_MAXEXPORTBATCHSIZE", "2048")

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
		},
		Output: Output{
			Type: OutputTypeOTLPGrpc,
			UDP:  UDPOutputConfig{Workers: 1},
			TCP: TCPOutputConfig{
				Workers:   1,
				EnableTLS: false,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
				},
			},
			OTLPGrpc: OTLPGrpcOutputConfig{
				Host:               "collector.example.com",
				Port:               4317,
				Workers:            5,
				BatchTimeout:       15 * time.Second,
				MaxQueueSize:       8192,
				MaxExportBatchSize: 2048,
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

func TestOverrideTCPTLSFlags(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	args := []string{
		"--output-type", "tcp",
		"--output-tcp-host", "127.0.0.1",
		"--output-tcp-port", "9090",
		"--output-tcp-workers", "3",
		"--output-tcp-enable-tls", "true",
		"--output-tcp-tls-cert", "/path/to/cert.pem",
		"--output-tcp-tls-key", "/path/to/key.pem",
		"--output-tcp-tls-ca", "/path/to/ca1.pem,/path/to/ca2.pem",
		"--output-tcp-tls-skip-verify", "true",
		"--output-tcp-tls-min-version", "1.2",
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
		Logging: Logging{Type: LoggingTypeStdout, Level: LogLevelInfo},
		Generator: Generator{
			JSON: JSONGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
		},
		Output: Output{
			Type: OutputTypeTCP,
			UDP:  UDPOutputConfig{Workers: 1},
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

func TestOverrideTCPTLSEnvs(t *testing.T) {
	t.Setenv("BLITZ_OUTPUT_TYPE", "tcp")
	t.Setenv("BLITZ_OUTPUT_TCP_HOST", "example.com")
	t.Setenv("BLITZ_OUTPUT_TCP_PORT", "8080")
	t.Setenv("BLITZ_OUTPUT_TCP_WORKERS", "2")
	t.Setenv("BLITZ_OUTPUT_TCP_ENABLE_TLS", "true")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_CERT", "/env/cert.pem")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_KEY", "/env/key.pem")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_CA", "/env/ca1.pem,/env/ca2.pem")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_SKIP_VERIFY", "true")
	t.Setenv("BLITZ_OUTPUT_TCP_TLS_MIN_VERSION", "1.3")

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
		},
		Output: Output{
			Type: OutputTypeTCP,
			UDP:  UDPOutputConfig{Workers: 1},
			TCP: TCPOutputConfig{
				Host:      "example.com",
				Port:      8080,
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
