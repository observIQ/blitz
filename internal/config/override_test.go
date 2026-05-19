package config

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// getTestOverrideFlagsArgs returns the flag arguments used in TestOverrideFlags.
// This function extracts the flags so they can be validated for coverage.
func getTestOverrideFlagsArgs() []string {
	return []string{
		"--logging-type", "stdout",
		"--logging-level", "warn",
		"--logging-file-path", "/test/logging/path.log",
		"--logging-file-rotation-maxsizemb", "50",
		"--logging-file-rotation-maxbackups", "5",
		"--logging-file-rotation-maxagedays", "10",
		"--logging-file-rotation-compress=false",
		"--logging-file-rotation-localtime=true",
		"--generator-count", "500",
		"--onfinish", "idle",
		"--generator-type", "json",
		"--generator-json-workers", "5",
		"--generator-json-rate", "500ms",
		"--generator-json-type", "pii",
		"--generator-winevt-workers", "4",
		"--generator-winevt-rate", "200ms",
		"--generator-paloalto-workers", "6",
		"--generator-paloalto-rate", "750ms",
		"--generator-apache-common-workers", "8",
		"--generator-apache-common-rate", "300ms",
		"--generator-apache-combined-workers", "10",
		"--generator-apache-combined-rate", "150ms",
		"--generator-apache-error-workers", "12",
		"--generator-apache-error-rate", "200ms",
		"--generator-nginx-workers", "14",
		"--generator-nginx-rate", "100ms",
		"--generator-postgres-workers", "16",
		"--generator-postgres-rate", "80ms",
		"--generator-kubernetes-workers", "18",
		"--generator-kubernetes-rate", "60ms",
		"--generator-kubernetes-format", "cri-o",
		"--generator-filegen-workers", "20",
		"--generator-filegen-rate", "50ms",
		"--generator-filegen-source", "/var/log",
		"--generator-filegen-cache-enabled=false",
		"--generator-filegen-cache-ttl", "0",
		"--generator-okta-workers", "22",
		"--generator-okta-rate", "40ms",
		"--generator-hostmetrics-workers", "3",
		"--generator-hostmetrics-rate", "5s",
		"--generator-hostmetrics-os", "windows",
		"--generator-hostmetrics-hostname", "test-host",
		"--generator-hostmetrics-scrapers", "cpu,memory",
		"--generator-traces-workers", "2",
		"--generator-traces-rate", "500ms",
		"--output-type", "otlp-grpc",
		"--output-udp-host", "udp.example.com",
		"--output-udp-port", "1514",
		"--output-udp-workers", "2",
		"--output-tcp-host", "127.0.0.1",
		"--output-tcp-port", "9090",
		"--output-tcp-workers", "3",
		"--output-tcp-enable-tls", "true",
		"--output-tcp-tls-cert", "/path/to/cert.pem",
		"--output-tcp-tls-key", "/path/to/key.pem",
		"--output-tcp-tls-ca", "/path/to/ca1.pem,/path/to/ca2.pem",
		"--output-tcp-tls-skip-verify", "true",
		"--output-tcp-tls-min-version", "1.2",
		"--output-syslog-host", "syslog.example.com",
		"--output-syslog-port", "5514",
		"--output-syslog-transport", "tcp",
		"--output-syslog-rfc", "3164",
		"--output-syslog-workers", "4",
		"--output-syslog-facility", "3",
		"--output-syslog-appname", "myapp",
		"--output-syslog-hostname", "myhost",
		"--output-syslog-procid", "pid42",
		"--output-syslog-msgid", "msg42",
		"--output-syslog-maxdatagrambytes", "1400",
		"--output-syslog-enable-tls", "true",
		"--output-syslog-tls-cert", "/path/to/syslog_cert.pem",
		"--output-syslog-tls-key", "/path/to/syslog_key.pem",
		"--output-syslog-tls-ca", "/path/to/sys_ca1.pem,/path/to/sys_ca2.pem",
		"--output-syslog-tls-skip-verify", "true",
		"--output-syslog-tls-min-version", "1.3",
		"--output-file-path", "/tmp/blitz.log",
		"--output-file-workers", "2",
		"--output-file-rotation-maxsizemb", "50",
		"--output-file-rotation-maxbackups", "5",
		"--output-file-rotation-maxagedays", "10",
		"--output-file-rotation-compress", "true",
		"--output-file-rotation-localtime", "true",
		"--output-otlpgrpc-host", "collector.example.com",
		"--output-otlpgrpc-port", "4317",
		"--output-otlpgrpc-workers", "3",
		"--output-otlpgrpc-batchtimeout", "10s",
		"--output-otlpgrpc-requesttimeout", "15s",
		"--output-otlpgrpc-maxqueuesize", "4096",
		"--output-otlpgrpc-maxexportbatchsize", "1024",
		"--output-otlpgrpc-enable-tls", "true",
		"--otlp-grpc-tls-insecure=false",
		"--otlp-grpc-tls-cert", "/path/to/otlp_cert.pem",
		"--otlp-grpc-tls-key", "/path/to/otlp_key.pem",
		"--otlp-grpc-tls-ca", "/path/to/otlp_ca1.pem,/path/to/otlp_ca2.pem",
		"--otlp-grpc-tls-skip-verify=false",
		"--otlp-grpc-tls-min-version", "1.3",
		"--output-hec-host", "hec.example.com",
		"--output-hec-port", "8088",
		"--output-hec-token", "my-hec-token",
		"--output-hec-workers", "2",
		"--output-hec-batchsize", "50",
		"--output-hec-batchtimeout", "3s",
		"--output-hec-eventformat", "parsed",
		"--output-hec-enableack", "true",
		"--output-hec-ackpollinterval", "5s",
		"--output-hec-acktimeout", "2m",
		"--output-hec-maxretries", "5",
		"--output-hec-source", "myapp",
		"--output-hec-sourcetype", "mylog",
		"--output-hec-index", "main",
		"--output-hec-enable-tls", "true",
		"--output-hec-tls-cert", "/path/to/hec_cert.pem",
		"--output-hec-tls-key", "/path/to/hec_key.pem",
		"--output-hec-tls-ca", "/path/to/hec_ca1.pem,/path/to/hec_ca2.pem",
		"--output-hec-tls-skip-verify", "true",
		"--output-hec-tls-min-version", "1.3",
		"--output-stdout-flushinterval", "50ms",
		"--metrics-port", "8080",
	}
}

// getTestOverrideEnvs returns the environment variables used in TestOverrideEnvs.
// This function extracts the env vars so they can be validated for coverage.
func getTestOverrideEnvs() map[string]string {
	return map[string]string{
		"BLITZ_LOGGING_TYPE":                       "stdout",
		"BLITZ_LOGGING_LEVEL":                      "error",
		"BLITZ_LOGGING_FILE_PATH":                  "/env/logging/path.log",
		"BLITZ_LOGGING_FILE_ROTATION_MAXSIZEMB":    "75",
		"BLITZ_LOGGING_FILE_ROTATION_MAXBACKUPS":   "6",
		"BLITZ_LOGGING_FILE_ROTATION_MAXAGEDAYS":   "20",
		"BLITZ_LOGGING_FILE_ROTATION_COMPRESS":     "false",
		"BLITZ_LOGGING_FILE_ROTATION_LOCALTIME":    "true",
		"BLITZ_GENERATOR_COUNT":                    "1000",
		"BLITZ_ONFINISH":                           "idle",
		"BLITZ_GENERATOR_TYPE":                     "winevt",
		"BLITZ_GENERATOR_JSON_WORKERS":             "3",
		"BLITZ_GENERATOR_JSON_RATE":                "250ms",
		"BLITZ_GENERATOR_JSON_TYPE":                "default",
		"BLITZ_GENERATOR_WINEVT_WORKERS":           "2",
		"BLITZ_GENERATOR_WINEVT_RATE":              "750ms",
		"BLITZ_GENERATOR_PALOALTO_WORKERS":         "7",
		"BLITZ_GENERATOR_PALOALTO_RATE":            "150ms",
		"BLITZ_GENERATOR_APACHE_COMMON_WORKERS":    "9",
		"BLITZ_GENERATOR_APACHE_COMMON_RATE":       "450ms",
		"BLITZ_GENERATOR_APACHE_COMBINED_WORKERS":  "11",
		"BLITZ_GENERATOR_APACHE_COMBINED_RATE":     "250ms",
		"BLITZ_GENERATOR_APACHE_ERROR_WORKERS":     "13",
		"BLITZ_GENERATOR_APACHE_ERROR_RATE":        "350ms",
		"BLITZ_GENERATOR_NGINX_WORKERS":            "15",
		"BLITZ_GENERATOR_NGINX_RATE":               "250ms",
		"BLITZ_GENERATOR_POSTGRES_WORKERS":         "17",
		"BLITZ_GENERATOR_POSTGRES_RATE":            "300ms",
		"BLITZ_GENERATOR_KUBERNETES_WORKERS":       "19",
		"BLITZ_GENERATOR_KUBERNETES_RATE":          "250ms",
		"BLITZ_GENERATOR_KUBERNETES_FORMAT":        "cri-o",
		"BLITZ_GENERATOR_FILEGEN_WORKERS":          "21",
		"BLITZ_GENERATOR_FILEGEN_RATE":             "45ms",
		"BLITZ_GENERATOR_FILEGEN_SOURCE":           "syslog_generic",
		"BLITZ_GENERATOR_FILEGEN_CACHE_ENABLED":    "false",
		"BLITZ_GENERATOR_FILEGEN_CACHE_TTL":        "0",
		"BLITZ_GENERATOR_OKTA_WORKERS":             "23",
		"BLITZ_GENERATOR_OKTA_RATE":                "35ms",
		"BLITZ_GENERATOR_HOSTMETRICS_WORKERS":      "4",
		"BLITZ_GENERATOR_HOSTMETRICS_RATE":         "10s",
		"BLITZ_GENERATOR_HOSTMETRICS_OS":           "linux",
		"BLITZ_GENERATOR_HOSTMETRICS_HOSTNAME":     "env-host",
		"BLITZ_GENERATOR_HOSTMETRICS_SCRAPERS":     "disk,network",
		"BLITZ_GENERATOR_TRACES_WORKERS":           "5",
		"BLITZ_GENERATOR_TRACES_RATE":              "2s",
		"BLITZ_OUTPUT_TYPE":                        "file",
		"BLITZ_OUTPUT_UDP_HOST":                    "udp.env.example",
		"BLITZ_OUTPUT_UDP_PORT":                    "5514",
		"BLITZ_OUTPUT_UDP_WORKERS":                 "4",
		"BLITZ_OUTPUT_TCP_HOST":                    "tcp.env.example",
		"BLITZ_OUTPUT_TCP_PORT":                    "8081",
		"BLITZ_OUTPUT_TCP_WORKERS":                 "2",
		"BLITZ_OUTPUT_TCP_ENABLE_TLS":              "true",
		"BLITZ_OUTPUT_TCP_TLS_CERT":                "/env/cert.pem",
		"BLITZ_OUTPUT_TCP_TLS_KEY":                 "/env/key.pem",
		"BLITZ_OUTPUT_TCP_TLS_CA":                  "/env/ca1.pem,/env/ca2.pem",
		"BLITZ_OUTPUT_TCP_TLS_SKIP_VERIFY":         "true",
		"BLITZ_OUTPUT_TCP_TLS_MIN_VERSION":         "1.3",
		"BLITZ_OUTPUT_SYSLOG_HOST":                 "syslog.env.example",
		"BLITZ_OUTPUT_SYSLOG_PORT":                 "6514",
		"BLITZ_OUTPUT_SYSLOG_TRANSPORT":            "tcp",
		"BLITZ_OUTPUT_SYSLOG_RFC":                  "5424",
		"BLITZ_OUTPUT_SYSLOG_WORKERS":              "6",
		"BLITZ_OUTPUT_SYSLOG_FACILITY":             "4",
		"BLITZ_OUTPUT_SYSLOG_APPNAME":              "envapp",
		"BLITZ_OUTPUT_SYSLOG_HOSTNAME":             "envhost",
		"BLITZ_OUTPUT_SYSLOG_PROCID":               "envpid",
		"BLITZ_OUTPUT_SYSLOG_MSGID":                "envmsg",
		"BLITZ_OUTPUT_SYSLOG_MAXDATAGRAMBYTES":     "1200",
		"BLITZ_OUTPUT_SYSLOG_ENABLE_TLS":           "true",
		"BLITZ_OUTPUT_SYSLOG_TLS_CERT":             "/env/syslog_cert.pem",
		"BLITZ_OUTPUT_SYSLOG_TLS_KEY":              "/env/syslog_key.pem",
		"BLITZ_OUTPUT_SYSLOG_TLS_CA":               "/env/sys_ca1.pem,/env/sys_ca2.pem",
		"BLITZ_OUTPUT_SYSLOG_TLS_SKIP_VERIFY":      "false",
		"BLITZ_OUTPUT_SYSLOG_TLS_MIN_VERSION":      "1.2",
		"BLITZ_OUTPUT_FILE_PATH":                   "/env/blitz.log",
		"BLITZ_OUTPUT_FILE_WORKERS":                "3",
		"BLITZ_OUTPUT_FILE_ROTATION_MAXSIZEMB":     "75",
		"BLITZ_OUTPUT_FILE_ROTATION_MAXBACKUPS":    "6",
		"BLITZ_OUTPUT_FILE_ROTATION_MAXAGEDAYS":    "20",
		"BLITZ_OUTPUT_FILE_ROTATION_COMPRESS":      "false",
		"BLITZ_OUTPUT_FILE_ROTATION_LOCALTIME":     "true",
		"BLITZ_OUTPUT_OTLPGRPC_HOST":               "collector.env.example",
		"BLITZ_OUTPUT_OTLPGRPC_PORT":               "4318",
		"BLITZ_OUTPUT_OTLPGRPC_WORKERS":            "5",
		"BLITZ_OUTPUT_OTLPGRPC_BATCHTIMEOUT":       "15s",
		"BLITZ_OUTPUT_OTLPGRPC_REQUESTTIMEOUT":     "20s",
		"BLITZ_OUTPUT_OTLPGRPC_MAXQUEUESIZE":       "8192",
		"BLITZ_OUTPUT_OTLPGRPC_MAXEXPORTBATCHSIZE": "2048",
		"BLITZ_OUTPUT_OTLPGRPC_ENABLE_TLS":         "true",
		"BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE":       "false",
		"BLITZ_OUTPUT_OTLPGRPC_TLS_CERT":           "/env/otlp_cert.pem",
		"BLITZ_OUTPUT_OTLPGRPC_TLS_KEY":            "/env/otlp_key.pem",
		"BLITZ_OUTPUT_OTLPGRPC_TLS_CA":             "/env/otlp_ca1.pem,/env/otlp_ca2.pem",
		"BLITZ_OUTPUT_OTLPGRPC_TLS_SKIP_VERIFY":    "false",
		"BLITZ_OUTPUT_OTLPGRPC_TLS_MIN_VERSION":    "1.2",
		"BLITZ_OUTPUT_HEC_HOST":                    "hec.env.example",
		"BLITZ_OUTPUT_HEC_PORT":                    "8089",
		"BLITZ_OUTPUT_HEC_TOKEN":                   "env-hec-token",
		"BLITZ_OUTPUT_HEC_WORKERS":                 "3",
		"BLITZ_OUTPUT_HEC_BATCHSIZE":               "75",
		"BLITZ_OUTPUT_HEC_BATCHTIMEOUT":            "7s",
		"BLITZ_OUTPUT_HEC_EVENTFORMAT":             "raw",
		"BLITZ_OUTPUT_HEC_ENABLEACK":               "false",
		"BLITZ_OUTPUT_HEC_ACKPOLLINTERVAL":         "15s",
		"BLITZ_OUTPUT_HEC_ACKTIMEOUT":              "3m",
		"BLITZ_OUTPUT_HEC_MAXRETRIES":              "2",
		"BLITZ_OUTPUT_HEC_SOURCE":                  "envapp",
		"BLITZ_OUTPUT_HEC_SOURCETYPE":              "envlog",
		"BLITZ_OUTPUT_HEC_INDEX":                   "dev",
		"BLITZ_OUTPUT_HEC_ENABLE_TLS":              "false",
		"BLITZ_OUTPUT_HEC_TLS_CERT":                "/env/hec_cert.pem",
		"BLITZ_OUTPUT_HEC_TLS_KEY":                 "/env/hec_key.pem",
		"BLITZ_OUTPUT_HEC_TLS_CA":                  "/env/hec_ca1.pem,/env/hec_ca2.pem",
		"BLITZ_OUTPUT_HEC_TLS_SKIP_VERIFY":         "true",
		"BLITZ_OUTPUT_HEC_TLS_MIN_VERSION":         "1.2",
		"BLITZ_OUTPUT_STDOUT_FLUSHINTERVAL":        "75ms",
		"BLITZ_METRICS_PORT":                       "9100",
	}
}

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
		Logging: Logging{
			Type:  LoggingTypeStdout,
			Level: LogLevelInfo,
			File: LoggingFileConfig{
				Path: DefaultLoggingFilePath,
				Rotation: FileRotationConfig{
					MaxSizeMB:  DefaultFileRotationMaxSizeMB,
					MaxBackups: DefaultFileRotationMaxBackups,
					MaxAgeDays: DefaultFileRotationMaxAgeDays,
					Compress:   true,
					LocalTime:  false,
				},
			},
		},
		Generator: Generator{
			Type:  GeneratorTypeNop,
			Count: 0,
			JSON: JSONGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
				Type:    "default",
			},
			Winevt: WinevtGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			PaloAlto: PaloAltoGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			Apache: ApacheGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			ApacheCombined: ApacheCombinedGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			ApacheError: ApacheErrorGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			Nginx: NginxGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			Postgres: PostgresGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			Kubernetes: KubernetesGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
				Format:  "cri-o",
			},
			Filegen: FileGeneratorConfig{
				Workers:      1,
				Rate:         1 * time.Second,
				Source:       "",
				CacheEnabled: true,
				CacheTTL:     0,
			},
			Okta: OktaGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
			HostMetrics: HostMetricsGeneratorConfig{
				Workers:  1,
				Rate:     1 * time.Second,
				OS:       "linux",
				Scrapers: []string{},
			},
			Traces: TracesGeneratorConfig{
				Workers: 1,
				Rate:    1 * time.Second,
			},
		},
		OnFinish: "exit",
		Output: Output{
			Type: OutputTypeNop,
			UDP:  UDPOutputConfig{Host: "", Port: 0, Workers: 1},
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
			Syslog: SyslogOutputConfig{
				Host:      "",
				Port:      0,
				Transport: SyslogTransport("udp"),
				RFC:       SyslogRFC("5424"),
				Workers:   1,
				Facility:  1,
				AppName:   "blitz",
				Hostname:  "",
				ProcID:    "",
				MsgID:     "",
				// default 0 means no truncation
				MaxDatagramBytes: 0,
				EnableTLS:        false,
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
				RequestTimeout:     DefaultOTLPGrpcRequestTimeout,
				MaxQueueSize:       DefaultOTLPGrpcMaxQueueSize,
				MaxExportBatchSize: DefaultOTLPGrpcMaxExportBatchSize,
				EnableTLS:          false,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
					Insecure:             true,
				},
			},
			HEC: HECOutputConfig{
				Port:            DefaultHECPort,
				Workers:         DefaultHECWorkers,
				BatchSize:       DefaultHECBatchSize,
				BatchTimeout:    DefaultHECBatchTimeout,
				EventFormat:     DefaultHECEventFormat,
				EnableACK:       DefaultHECEnableACK,
				ACKPollInterval: DefaultHECACKPollInterval,
				ACKTimeout:      DefaultHECACKTimeout,
				MaxRetries:      DefaultHECMaxRetries,
				Source:          DefaultHECSource,
				SourceType:      DefaultHECSourceType,
				EnableTLS:       DefaultHECEnableTLS,
				TLS: TLS{
					MinTLSVersion:        "1.2",
					CertificateAuthority: []string{},
				},
			},
			Stdout: StdoutOutputConfig{
				FlushInterval: DefaultStdoutFlushInterval,
			},
		},
		Metrics: Metrics{
			Port: DefaultMetricsPort,
		},
	}
	require.Equal(t, expectedCfg, cfg)
}

func TestOverrideFlags(t *testing.T) {
	// Ensure environment variables cannot interfere with flag-based expectations
	// Unset these env vars to prevent interference with flag parsing
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE", "")
	t.Setenv("BLITZ_OUTPUT_OTLPGRPC_TLS_SKIP_VERIFY", "")

	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	args := getTestOverrideFlagsArgs()

	overrides := DefaultOverrides()
	for _, override := range overrides {
		require.NoError(t, override.Bind(flagSet))
	}

	require.NoError(t, flagSet.Parse(args))

	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// NOTE: Do NOT call viper.AutomaticEnv() here - we're testing flag overrides without environment variables

	cfg := NewConfig()
	err := viper.Unmarshal(cfg)
	require.NoError(t, err)

	// build expected config and compare full struct
	expectedCfg := &Config{
		Logging: Logging{
			Type:  LoggingTypeStdout,
			Level: LogLevelWarn,
			File: LoggingFileConfig{
				Path: "/test/logging/path.log",
				Rotation: FileRotationConfig{
					MaxSizeMB:  50,
					MaxBackups: 5,
					MaxAgeDays: 10,
					Compress:   false,
					LocalTime:  true,
				},
			},
		},
		Generator: Generator{
			Type:  GeneratorTypeJSON,
			Count: 500,
			JSON: JSONGeneratorConfig{
				Workers: 5,
				Rate:    500 * time.Millisecond,
				Type:    "pii",
			},
			Winevt: WinevtGeneratorConfig{
				Workers: 4,
				Rate:    200 * time.Millisecond,
			},
			PaloAlto: PaloAltoGeneratorConfig{
				Workers: 6,
				Rate:    750 * time.Millisecond,
			},
			Apache: ApacheGeneratorConfig{
				Workers: 8,
				Rate:    300 * time.Millisecond,
			},
			ApacheCombined: ApacheCombinedGeneratorConfig{
				Workers: 10,
				Rate:    150 * time.Millisecond,
			},
			ApacheError: ApacheErrorGeneratorConfig{
				Workers: 12,
				Rate:    200 * time.Millisecond,
			},
			Nginx: NginxGeneratorConfig{
				Workers: 14,
				Rate:    100 * time.Millisecond,
			},
			Postgres: PostgresGeneratorConfig{
				Workers: 16,
				Rate:    80 * time.Millisecond,
			},
			Kubernetes: KubernetesGeneratorConfig{
				Workers: 18,
				Rate:    60 * time.Millisecond,
				Format:  "cri-o",
			},
			Filegen: FileGeneratorConfig{
				Workers:      20,
				Rate:         50 * time.Millisecond,
				Source:       "/var/log",
				CacheEnabled: false,
				CacheTTL:     0,
			},
			Okta: OktaGeneratorConfig{
				Workers: 22,
				Rate:    40 * time.Millisecond,
			},
			HostMetrics: HostMetricsGeneratorConfig{
				Workers:  3,
				Rate:     5 * time.Second,
				OS:       "windows",
				Hostname: "test-host",
				Scrapers: []string{"cpu", "memory"},
			},
			Traces: TracesGeneratorConfig{
				Workers: 2,
				Rate:    500 * time.Millisecond,
			},
		},
		OnFinish: "idle",
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
			Syslog: SyslogOutputConfig{
				Host:      "syslog.example.com",
				Port:      5514,
				Transport: SyslogTransport("tcp"),
				RFC:       SyslogRFC("3164"),
				Workers:   4,
				Facility:  3,
				AppName:   "myapp",
				Hostname:  "myhost",
				ProcID:    "pid42",
				MsgID:     "msg42",
				// set even though TCP ignores it; present for completeness
				MaxDatagramBytes: 1400,
				EnableTLS:        true,
				TLS: TLS{
					Certificate:          "/path/to/syslog_cert.pem",
					PrivateKey:           "/path/to/syslog_key.pem",
					CertificateAuthority: []string{"/path/to/sys_ca1.pem", "/path/to/sys_ca2.pem"},
					InsecureSkipVerify:   true,
					MinTLSVersion:        "1.3",
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
				RequestTimeout:     15 * time.Second,
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
			HEC: HECOutputConfig{
				Host:            "hec.example.com",
				Port:            8088,
				Token:           "my-hec-token",
				Workers:         2,
				BatchSize:       50,
				BatchTimeout:    3 * time.Second,
				EventFormat:     "parsed",
				EnableACK:       true,
				ACKPollInterval: 5 * time.Second,
				ACKTimeout:      2 * time.Minute,
				MaxRetries:      5,
				Source:          "myapp",
				SourceType:      "mylog",
				Index:           "main",
				EnableTLS:       true,
				TLS: TLS{
					Certificate:          "/path/to/hec_cert.pem",
					PrivateKey:           "/path/to/hec_key.pem",
					CertificateAuthority: []string{"/path/to/hec_ca1.pem", "/path/to/hec_ca2.pem"},
					InsecureSkipVerify:   true,
					MinTLSVersion:        "1.3",
				},
			},
			Stdout: StdoutOutputConfig{
				FlushInterval: 50 * time.Millisecond,
			},
		},
		Metrics: Metrics{
			Port: 8080,
		},
	}
	require.Equal(t, expectedCfg, cfg)
}

func TestOverrideEnvs(t *testing.T) {
	envs := getTestOverrideEnvs()
	setEnvs(t, envs)

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
		Logging: Logging{
			Type:  LoggingTypeStdout,
			Level: LogLevelError,
			File: LoggingFileConfig{
				Path: "/env/logging/path.log",
				Rotation: FileRotationConfig{
					MaxSizeMB:  75,
					MaxBackups: 6,
					MaxAgeDays: 20,
					Compress:   false,
					LocalTime:  true,
				},
			},
		},
		Generator: Generator{
			Type:  GeneratorTypeWinevt,
			Count: 1000,
			JSON: JSONGeneratorConfig{
				Workers: 3,
				Rate:    250 * time.Millisecond,
				Type:    "default",
			},
			Winevt: WinevtGeneratorConfig{
				Workers: 2,
				Rate:    750 * time.Millisecond,
			},
			PaloAlto: PaloAltoGeneratorConfig{
				Workers: 7,
				Rate:    150 * time.Millisecond,
			},
			Apache: ApacheGeneratorConfig{
				Workers: 9,
				Rate:    450 * time.Millisecond,
			},
			ApacheCombined: ApacheCombinedGeneratorConfig{
				Workers: 11,
				Rate:    250 * time.Millisecond,
			},
			ApacheError: ApacheErrorGeneratorConfig{
				Workers: 13,
				Rate:    350 * time.Millisecond,
			},
			Nginx: NginxGeneratorConfig{
				Workers: 15,
				Rate:    250 * time.Millisecond,
			},
			Postgres: PostgresGeneratorConfig{
				Workers: 17,
				Rate:    300 * time.Millisecond,
			},
			Kubernetes: KubernetesGeneratorConfig{
				Workers: 19,
				Rate:    250 * time.Millisecond,
				Format:  "cri-o",
			},
			Filegen: FileGeneratorConfig{
				Workers:      21,
				Rate:         45 * time.Millisecond,
				Source:       "syslog_generic",
				CacheEnabled: false,
				CacheTTL:     0,
			},
			Okta: OktaGeneratorConfig{
				Workers: 23,
				Rate:    35 * time.Millisecond,
			},
			HostMetrics: HostMetricsGeneratorConfig{
				Workers:  4,
				Rate:     10 * time.Second,
				OS:       "linux",
				Hostname: "env-host",
				Scrapers: []string{"disk", "network"},
			},
			Traces: TracesGeneratorConfig{
				Workers: 5,
				Rate:    2 * time.Second,
			},
		},
		OnFinish: "idle",
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
			Syslog: SyslogOutputConfig{
				Host:             "syslog.env.example",
				Port:             6514,
				Transport:        SyslogTransport("tcp"),
				RFC:              SyslogRFC("5424"),
				Workers:          6,
				Facility:         4,
				AppName:          "envapp",
				Hostname:         "envhost",
				ProcID:           "envpid",
				MsgID:            "envmsg",
				MaxDatagramBytes: 1200,
				EnableTLS:        true,
				TLS: TLS{
					Certificate:          "/env/syslog_cert.pem",
					PrivateKey:           "/env/syslog_key.pem",
					CertificateAuthority: []string{"/env/sys_ca1.pem", "/env/sys_ca2.pem"},
					InsecureSkipVerify:   false,
					MinTLSVersion:        "1.2",
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
				RequestTimeout:     20 * time.Second,
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
			HEC: HECOutputConfig{
				Host:            "hec.env.example",
				Port:            8089,
				Token:           "env-hec-token",
				Workers:         3,
				BatchSize:       75,
				BatchTimeout:    7 * time.Second,
				EventFormat:     "raw",
				EnableACK:       false,
				ACKPollInterval: 15 * time.Second,
				ACKTimeout:      3 * time.Minute,
				MaxRetries:      2,
				Source:          "envapp",
				SourceType:      "envlog",
				Index:           "dev",
				EnableTLS:       false,
				TLS: TLS{
					Certificate:          "/env/hec_cert.pem",
					PrivateKey:           "/env/hec_key.pem",
					CertificateAuthority: []string{"/env/hec_ca1.pem", "/env/hec_ca2.pem"},
					InsecureSkipVerify:   true,
					MinTLSVersion:        "1.2",
				},
			},
			Stdout: StdoutOutputConfig{
				FlushInterval: 75 * time.Millisecond,
			},
		},
		Metrics: Metrics{
			Port: 9100,
		},
	}
	require.Equal(t, expectedCfg, cfg)
}

// setEnvs sets the given environment variables.
func setEnvs(t *testing.T, envs map[string]string) {
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

// TestOverrideCoverage validates that all configuration overrides are tested in
// TestOverrideFlags and TestOverrideEnvs. This test ensures complete coverage
// and helps prevent missing test cases when new overrides are added.
func TestOverrideCoverage(t *testing.T) {
	allOverrides := DefaultOverrides()
	testedFlags := getTestOverrideFlagsArgs()
	testedEnvs := getTestOverrideEnvs()

	// Build sets of tested flags and env vars for quick lookup
	testedFlagSet := make(map[string]bool)
	for i := range testedFlags {
		if after, ok := strings.CutPrefix(testedFlags[i], "--"); ok {
			flagName := after
			// Handle both --flag value and --flag=value formats
			if idx := strings.Index(flagName, "="); idx != -1 {
				flagName = flagName[:idx]
			}
			testedFlagSet[flagName] = true
		}
	}

	testedEnvSet := make(map[string]bool)
	for env := range testedEnvs {
		testedEnvSet[env] = true
	}

	// Track missing coverage
	var missingFlags []string
	var missingEnvs []string

	// Check each override
	for _, override := range allOverrides {
		// Check flag coverage
		if !testedFlagSet[override.Flag] {
			missingFlags = append(missingFlags, fmt.Sprintf("  - Flag: %s (Field: %s, Env: %s)", override.Flag, override.Field, override.Env))
		}

		// Check env coverage
		if !testedEnvSet[override.Env] {
			missingEnvs = append(missingEnvs, fmt.Sprintf("  - Env: %s (Field: %s, Flag: %s)", override.Env, override.Field, override.Flag))
		}
	}

	// Report any missing coverage
	if len(missingFlags) > 0 || len(missingEnvs) > 0 {
		var report strings.Builder
		report.WriteString("Missing test coverage detected:\n\n")

		if len(missingFlags) > 0 {
			report.WriteString(fmt.Sprintf("Missing flags in TestOverrideFlags (%d):\n", len(missingFlags)))
			for _, missing := range missingFlags {
				report.WriteString(missing)
				report.WriteString("\n")
			}
			report.WriteString("\n")
		}

		if len(missingEnvs) > 0 {
			report.WriteString(fmt.Sprintf("Missing env vars in TestOverrideEnvs (%d):\n", len(missingEnvs)))
			for _, missing := range missingEnvs {
				report.WriteString(missing)
				report.WriteString("\n")
			}
		}

		t.Errorf("%s", report.String())
	}
}
