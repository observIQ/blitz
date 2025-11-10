package config

import (
	"strings"
	"time"

	"github.com/observiq/blitz/internal/generator/logtypes"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Override is a configuration override
type Override struct {
	// Field is the config field to override
	Field string
	// Flag is the flag that will override the field
	Flag string
	// Env is the environment variable that will override the field
	Env string
	// Usage is the usage for the override
	Usage string
	// Default is the default value for the override
	Default any
}

// NewOverride creates a new override
func NewOverride(field, usage string, def any) *Override {
	return &Override{
		Field:   field,
		Flag:    createFlagName(field),
		Env:     createEnvName(field),
		Usage:   usage,
		Default: def,
	}
}

// Bind binds the override to the viper instance
func (o *Override) Bind(flags *pflag.FlagSet) error {
	flag := o.createFlag(flags)
	if err := viper.BindPFlag(o.Field, flag); err != nil {
		return err
	}
	if err := viper.BindEnv(o.Field, o.Env); err != nil {
		return err
	}
	return nil
}

// createFlag creates a flag for the override
func (o *Override) createFlag(flags *pflag.FlagSet) *pflag.Flag {
	if exitingFlag := flags.Lookup(o.Flag); exitingFlag != nil {
		return exitingFlag
	}

	// Set the default value into Viper; flags act only as overrides.
	viper.SetDefault(o.Field, o.Default)

	switch o.Default.(type) {
	case string:
		_ = flags.String(o.Flag, "", o.Usage)
	case []string:
		_ = flags.StringSlice(o.Flag, []string{}, o.Usage)
	case LogLevel:
		_ = flags.String(o.Flag, "", o.Usage)
	case int:
		_ = flags.Int(o.Flag, 0, o.Usage)
	case time.Duration:
		_ = flags.Duration(o.Flag, 0, o.Usage)
	case bool:
		_ = flags.Bool(o.Flag, false, o.Usage)
	default:
		_ = flags.String(o.Flag, "", o.Usage)
	}

	return flags.Lookup(o.Flag)
}

// createFlagName creates a flag name from a field
func createFlagName(field string) string {
	updatedField := strings.ReplaceAll(field, ".", "-")
	return strings.ToLower(updatedField)
}

// createEnvName creates an environment variable name from a field
func createEnvName(field string) string {
	updatedField := strings.ReplaceAll(field, ".", "_")
	updatedField = strings.ReplaceAll(updatedField, "-", "_")
	updatedField = strings.ToUpper(updatedField)
	return "BLITZ_" + updatedField
}

// tcpTLSOverrides creates TCP TLS overrides that removes double tls-tls in flag name
func tcpTLSOverrides() []*Override {
	return []*Override{
		{
			Field:   "output.tcp.enableTLS",
			Flag:    "output-tcp-enable-tls",
			Env:     "BLITZ_OUTPUT_TCP_ENABLE_TLS",
			Usage:   "enable TLS for TCP connections",
			Default: false,
		},
		{
			Field:   "output.tcp.tls.cert",
			Flag:    "output-tcp-tls-cert",
			Env:     "BLITZ_OUTPUT_TCP_TLS_CERT",
			Usage:   "the path to the TLS certificate for TCP connections",
			Default: "",
		},
		{
			Field:   "output.tcp.tls.key",
			Flag:    "output-tcp-tls-key",
			Env:     "BLITZ_OUTPUT_TCP_TLS_KEY",
			Usage:   "the path to the TLS private key for TCP connections",
			Default: "",
		},
		{
			Field:   "output.tcp.tls.ca",
			Flag:    "output-tcp-tls-ca",
			Env:     "BLITZ_OUTPUT_TCP_TLS_CA",
			Usage:   "the path to the TLS CA files. Optional, if not provided the host's root CA set will be used",
			Default: []string{},
		},
		{
			Field:   "output.tcp.tls.skipVerify",
			Flag:    "output-tcp-tls-skip-verify",
			Env:     "BLITZ_OUTPUT_TCP_TLS_SKIP_VERIFY",
			Usage:   "whether to skip TLS verification for TCP connections",
			Default: false,
		},
		{
			Field:   "output.tcp.tls.minVersion",
			Flag:    "output-tcp-tls-min-version",
			Env:     "BLITZ_OUTPUT_TCP_TLS_MIN_VERSION",
			Usage:   "the minimum TLS version to use for TCP connections. One of: 1.2|1.3",
			Default: "1.2",
		},
	}
}

// otlpGrpcTLSOverrides creates OTLP gRPC TLS overrides
func otlpGrpcTLSOverrides() []*Override {
	return []*Override{
		{
			Field:   "output.otlpGrpc.enableTLS",
			Flag:    "output-otlpgrpc-enable-tls",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_ENABLE_TLS",
			Usage:   "enable TLS for OTLP gRPC connections",
			Default: false,
		},
		{
			Field:   "output.otlpGrpc.tls.insecure",
			Flag:    "otlp-grpc-tls-insecure",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE",
			Usage:   "whether to use insecure credentials (no TLS) for OTLP gRPC connections",
			Default: true,
		},
		// Also bind flattened keys for inline TLS fields, using the same flags
		{
			Field:   "output.otlpGrpc.insecure",
			Flag:    "otlp-grpc-tls-insecure",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE",
			Usage:   "whether to use insecure credentials (no TLS) for OTLP gRPC connections",
			Default: true,
		},
		{
			Field:   "output.otlpGrpc.tls.cert",
			Flag:    "otlp-grpc-tls-cert",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_CERT",
			Usage:   "the path to the TLS certificate for OTLP gRPC connections",
			Default: "",
		},
		{
			Field:   "output.otlpGrpc.cert",
			Flag:    "otlp-grpc-tls-cert",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_CERT",
			Usage:   "the path to the TLS certificate for OTLP gRPC connections",
			Default: "",
		},
		{
			Field:   "output.otlpGrpc.tls.key",
			Flag:    "otlp-grpc-tls-key",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_KEY",
			Usage:   "the path to the TLS private key for OTLP gRPC connections",
			Default: "",
		},
		{
			Field:   "output.otlpGrpc.key",
			Flag:    "otlp-grpc-tls-key",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_KEY",
			Usage:   "the path to the TLS private key for OTLP gRPC connections",
			Default: "",
		},
		{
			Field:   "output.otlpGrpc.tls.ca",
			Flag:    "otlp-grpc-tls-ca",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_CA",
			Usage:   "the path to the TLS CA files. Optional, if not provided the host's root CA set will be used",
			Default: []string{},
		},
		{
			Field:   "output.otlpGrpc.ca",
			Flag:    "otlp-grpc-tls-ca",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_CA",
			Usage:   "the path to the TLS CA files. Optional, if not provided the host's root CA set will be used",
			Default: []string{},
		},
		{
			Field:   "output.otlpGrpc.tls.skipVerify",
			Flag:    "otlp-grpc-tls-skip-verify",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_SKIP_VERIFY",
			Usage:   "whether to skip TLS verification for OTLP gRPC connections",
			Default: false,
		},
		{
			Field:   "output.otlpGrpc.skipVerify",
			Flag:    "otlp-grpc-tls-skip-verify",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_SKIP_VERIFY",
			Usage:   "whether to skip TLS verification for OTLP gRPC connections",
			Default: false,
		},
		{
			Field:   "output.otlpGrpc.tls.minVersion",
			Flag:    "otlp-grpc-tls-min-version",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_MIN_VERSION",
			Usage:   "the minimum TLS version to use for OTLP gRPC connections. One of: 1.2|1.3",
			Default: "1.2",
		},
		{
			Field:   "output.otlpGrpc.minVersion",
			Flag:    "otlp-grpc-tls-min-version",
			Env:     "BLITZ_OUTPUT_OTLPGRPC_TLS_MIN_VERSION",
			Usage:   "the minimum TLS version to use for OTLP gRPC connections. One of: 1.2|1.3",
			Default: "1.2",
		},
	}
}

// DefaultOverrides returns all overrides for the application
func DefaultOverrides() []*Override {
	overrides := []*Override{
		NewOverride("logging.type", "output of the log. One of: stdout|file", LoggingTypeStdout),
		NewOverride("logging.level", "log level to use. One of: debug|info|warn|error", LogLevelInfo),
		NewOverride("logging.file.path", "file path for file logging", DefaultLoggingFilePath),
		NewOverride("logging.file.rotation.maxSizeMB", "logging file rotation: maximum size in MB before rotation", DefaultFileRotationMaxSizeMB),
		NewOverride("logging.file.rotation.maxBackups", "logging file rotation: maximum number of backups to retain", DefaultFileRotationMaxBackups),
		NewOverride("logging.file.rotation.maxAgeDays", "logging file rotation: maximum age in days to retain backups", DefaultFileRotationMaxAgeDays),
		NewOverride("logging.file.rotation.compress", "logging file rotation: compress rotated files", true),
		NewOverride("logging.file.rotation.localTime", "logging file rotation: use local time for backup timestamps", false),
		NewOverride("generator.type", "generator type. One of: nop|json|winevt|palo-alto|apache-common|apache-combined|apache-error|nginx|postgres|kubernetes", GeneratorTypeNop),
		NewOverride("generator.json.workers", "number of JSON generator workers", 1),
		NewOverride("generator.json.rate", "rate at which logs are generated per worker", 1*time.Second),
		NewOverride("generator.json.type", "type of log to generate. One of: default|pii", logtypes.LogTypeDefault),
		NewOverride("generator.winevt.workers", "number of winevt generator workers", 1),
		NewOverride("generator.winevt.rate", "rate at which winevt logs are generated per worker", 1*time.Second),
		NewOverride("generator.paloAlto.workers", "number of palo-alto generator workers", 1),
		NewOverride("generator.paloAlto.rate", "rate at which palo-alto logs are generated per worker", 1*time.Second),
		NewOverride("generator.apache-common.workers", "number of Apache Common generator workers", 1),
		NewOverride("generator.apache-common.rate", "rate at which Apache Common logs are generated per worker", 1*time.Second),
		NewOverride("generator.apache-combined.workers", "number of Apache Combined generator workers", 1),
		NewOverride("generator.apache-combined.rate", "rate at which Apache Combined logs are generated per worker", 1*time.Second),
		NewOverride("generator.apache-error.workers", "number of Apache Error generator workers", 1),
		NewOverride("generator.apache-error.rate", "rate at which Apache Error logs are generated per worker", 1*time.Second),
		NewOverride("generator.nginx.workers", "number of NGINX generator workers", 1),
		NewOverride("generator.nginx.rate", "rate at which NGINX logs are generated per worker", 1*time.Second),
		NewOverride("generator.postgres.workers", "number of PostgreSQL generator workers", 1),
		NewOverride("generator.postgres.rate", "rate at which PostgreSQL logs are generated per worker", 1*time.Second),
		NewOverride("generator.kubernetes.workers", "number of Kubernetes generator workers", 1),
		NewOverride("generator.kubernetes.rate", "rate at which Kubernetes logs are generated per worker", 1*time.Second),
		NewOverride("generator.kubernetes.format", "container log format. One of: cri-o", KubernetesFormatCRIO),
		NewOverride("output.type", "output type. One of: nop|stdout|tcp|udp|syslog|otlp-grpc|file", OutputTypeNop),
		NewOverride("output.udp.host", "UDP output target host", ""),
		NewOverride("output.udp.port", "UDP output target port", 0),
		NewOverride("output.udp.workers", "number of UDP output workers", 1),
		NewOverride("output.tcp.host", "TCP output target host", ""),
		NewOverride("output.tcp.port", "TCP output target port", 0),
		NewOverride("output.tcp.workers", "number of TCP output workers", 1),
		NewOverride("output.syslog.host", "Syslog output target host", ""),
		NewOverride("output.syslog.port", "Syslog output target port", 0),
		NewOverride("output.syslog.transport", "Syslog transport. One of: tcp|udp", "udp"),
		NewOverride("output.syslog.rfc", "Syslog RFC format. One of: 3164|5424", "5424"),
		NewOverride("output.syslog.workers", "number of Syslog output workers", 1),
		NewOverride("output.syslog.facility", "Syslog facility (0-23)", 1),
		NewOverride("output.syslog.appName", "Syslog app name", "blitz"),
		NewOverride("output.syslog.hostname", "Syslog hostname", ""),
		NewOverride("output.syslog.procId", "Syslog process id", ""),
		NewOverride("output.syslog.msgId", "Syslog message id", ""),
		NewOverride("output.syslog.maxDatagramBytes", "Syslog UDP max datagram size (bytes). If <=0, no truncation", 0),
		NewOverride("output.file.path", "File output destination path", ""),
		NewOverride("output.file.workers", "number of File output workers", DefaultFileWorkers),
		NewOverride("output.file.rotation.maxSizeMB", "File output rotation: maximum size in MB before rotation", DefaultFileRotationMaxSizeMB),
		NewOverride("output.file.rotation.maxBackups", "File output rotation: maximum number of backups to retain", DefaultFileRotationMaxBackups),
		NewOverride("output.file.rotation.maxAgeDays", "File output rotation: maximum age in days to retain backups", DefaultFileRotationMaxAgeDays),
		NewOverride("output.file.rotation.compress", "File output rotation: compress rotated files", true),
		NewOverride("output.file.rotation.localTime", "File output rotation: use local time for backup timestamps", false),
		NewOverride("output.otlpGrpc.host", "OTLP gRPC output target host", DefaultOTLPGrpcHost),
		NewOverride("output.otlpGrpc.port", "OTLP gRPC output target port", DefaultOTLPGrpcPort),
		NewOverride("output.otlpGrpc.workers", "number of OTLP gRPC output workers", DefaultOTLPGrpcWorkers),
		NewOverride("output.otlpGrpc.batchTimeout", "OTLP gRPC output batch timeout", DefaultOTLPGrpcBatchTimeout),
		NewOverride("output.otlpGrpc.maxQueueSize", "OTLP gRPC output maximum queue size", DefaultOTLPGrpcMaxQueueSize),
		NewOverride("output.otlpGrpc.maxExportBatchSize", "OTLP gRPC output maximum export batch size", DefaultOTLPGrpcMaxExportBatchSize),
	}

	overrides = append(overrides, tcpTLSOverrides()...)
	overrides = append(overrides, syslogTLSOverrides()...)
	overrides = append(overrides, otlpGrpcTLSOverrides()...)
	return overrides
}

// syslogTLSOverrides creates Syslog TLS overrides
func syslogTLSOverrides() []*Override {
	return []*Override{
		{
			Field:   "output.syslog.enableTLS",
			Flag:    "output-syslog-enable-tls",
			Env:     "BLITZ_OUTPUT_SYSLOG_ENABLE_TLS",
			Usage:   "enable TLS for Syslog connections (transport tcp only)",
			Default: false,
		},
		{
			Field:   "output.syslog.tls.cert",
			Flag:    "output-syslog-tls-cert",
			Env:     "BLITZ_OUTPUT_SYSLOG_TLS_CERT",
			Usage:   "the path to the TLS certificate for Syslog TCP connections",
			Default: "",
		},
		{
			Field:   "output.syslog.tls.key",
			Flag:    "output-syslog-tls-key",
			Env:     "BLITZ_OUTPUT_SYSLOG_TLS_KEY",
			Usage:   "the path to the TLS private key for Syslog TCP connections",
			Default: "",
		},
		{
			Field:   "output.syslog.tls.ca",
			Flag:    "output-syslog-tls-ca",
			Env:     "BLITZ_OUTPUT_SYSLOG_TLS_CA",
			Usage:   "the path to the TLS CA files for Syslog TCP connections",
			Default: []string{},
		},
		{
			Field:   "output.syslog.tls.skipVerify",
			Flag:    "output-syslog-tls-skip-verify",
			Env:     "BLITZ_OUTPUT_SYSLOG_TLS_SKIP_VERIFY",
			Usage:   "whether to skip TLS verification for Syslog TCP connections",
			Default: false,
		},
		{
			Field:   "output.syslog.tls.minVersion",
			Flag:    "output-syslog-tls-min-version",
			Env:     "BLITZ_OUTPUT_SYSLOG_TLS_MIN_VERSION",
			Usage:   "the minimum TLS version to use for Syslog TCP connections. One of: 1.2|1.3",
			Default: "1.2",
		},
	}
}
