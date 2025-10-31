package config

import (
	"strings"
	"time"

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

	switch v := o.Default.(type) {
	case string:
		_ = flags.String(o.Flag, v, o.Usage)
	case []string:
		_ = flags.StringSlice(o.Flag, v, o.Usage)
	case LogLevel:
		_ = flags.String(o.Flag, string(v), o.Usage)
	case int:
		_ = flags.Int(o.Flag, v, o.Usage)
	case time.Duration:
		_ = flags.Duration(o.Flag, v, o.Usage)
	case bool:
		_ = flags.Bool(o.Flag, v, o.Usage)
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
		{
			Field:   "output.otlpGrpc.tls.cert",
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
			Field:   "output.otlpGrpc.tls.ca",
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
			Field:   "output.otlpGrpc.tls.minVersion",
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
		NewOverride("logging.type", "output of the log. One of: stdout", LoggingTypeStdout),
		NewOverride("logging.level", "log level to use. One of: debug|info|warn|error", LogLevelInfo),
		NewOverride("generator.type", "generator type. One of: nop|json|winevt", GeneratorTypeNop),
		NewOverride("generator.json.workers", "number of JSON generator workers", 1),
		NewOverride("generator.json.rate", "rate at which logs are generated per worker", 1*time.Second),
		NewOverride("generator.winevt.workers", "number of winevt generator workers", 1),
		NewOverride("generator.winevt.rate", "rate at which winevt logs are generated per worker", 1*time.Second),
		NewOverride("output.type", "output type. One of: nop|tcp|udp|otlp-grpc|s3", OutputTypeNop),
		NewOverride("output.udp.host", "UDP output target host", ""),
		NewOverride("output.udp.port", "UDP output target port", 0),
		NewOverride("output.udp.workers", "number of UDP output workers", 1),
		NewOverride("output.tcp.host", "TCP output target host", ""),
		NewOverride("output.tcp.port", "TCP output target port", 0),
		NewOverride("output.tcp.workers", "number of TCP output workers", 1),
		NewOverride("output.otlpGrpc.host", "OTLP gRPC output target host", DefaultOTLPGrpcHost),
		NewOverride("output.otlpGrpc.port", "OTLP gRPC output target port", DefaultOTLPGrpcPort),
		NewOverride("output.otlpGrpc.workers", "number of OTLP gRPC output workers", DefaultOTLPGrpcWorkers),
		NewOverride("output.otlpGrpc.batchTimeout", "OTLP gRPC output batch timeout", DefaultOTLPGrpcBatchTimeout),
		NewOverride("output.otlpGrpc.maxQueueSize", "OTLP gRPC output maximum queue size", DefaultOTLPGrpcMaxQueueSize),
		NewOverride("output.otlpGrpc.maxExportBatchSize", "OTLP gRPC output maximum export batch size", DefaultOTLPGrpcMaxExportBatchSize),
		// S3 output
		NewOverride("output.s3.bucket", "S3 output bucket name", ""),
		NewOverride("output.s3.region", "S3 output AWS region", DefaultS3Region),
		NewOverride("output.s3.keyPrefix", "S3 output key prefix for uploaded objects (optional)", ""),
		NewOverride("output.s3.workers", "number of S3 output workers", DefaultS3Workers),
		NewOverride("output.s3.batchTimeout", "S3 output batch timeout", DefaultS3BatchTimeout),
		NewOverride("output.s3.batchSize", "S3 output logs per batch", DefaultS3BatchSize),
		NewOverride("output.s3.accessKeyID", "S3 output AWS access key ID (optional)", ""),
		NewOverride("output.s3.secretAccessKey", "S3 output AWS secret access key (optional)", ""),
	}

	overrides = append(overrides, tcpTLSOverrides()...)
	overrides = append(overrides, otlpGrpcTLSOverrides()...)
	return overrides
}
