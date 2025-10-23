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
	case LogLevel:
		_ = flags.String(o.Flag, string(v), o.Usage)
	case int:
		_ = flags.Int(o.Flag, v, o.Usage)
	case time.Duration:
		_ = flags.Duration(o.Flag, v, o.Usage)
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
	return "BINDPLANE_" + updatedField
}

// DefaultOverrides returns all overrides for the application
func DefaultOverrides() []*Override {
	return []*Override{
		NewOverride("logging.type", "output of the log. One of: stdout", LoggingTypeStdout),
		NewOverride("logging.level", "log level to use. One of: debug|info|warn|error", LogLevelInfo),
		NewOverride("generator.type", "generator type. One of: json", ""),
		NewOverride("generator.json.workers", "number of JSON generator workers", 1),
		NewOverride("generator.json.rate", "rate at which logs are generated per worker", 1*time.Second),
		NewOverride("output.type", "output type. One of: tcp|udp", ""),
		NewOverride("output.udp.host", "UDP output target host", ""),
		NewOverride("output.udp.port", "UDP output target port", 0),
		NewOverride("output.udp.workers", "number of UDP output workers", 1),
		NewOverride("output.tcp.host", "TCP output target host", ""),
		NewOverride("output.tcp.port", "TCP output target port", 0),
		NewOverride("output.tcp.workers", "number of TCP output workers", 1),
	}
}
