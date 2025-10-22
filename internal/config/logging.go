package config

import (
	"errors"
	"fmt"
	"strings"
)

// LogLevel represents a supported logging severity level.
type LogLevel string

const (
	// LoggingTypeStdout writes logs to stdout (only supported type).
	LoggingTypeStdout = "stdout"

	// LogLevelDebug is the debug log level.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo is the info log level.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn is the warn log level.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError is the error log level.
	LogLevelError LogLevel = "error"
)

var (
	// errInvalidLoggingType is returned when an invalid logging type is provided.
	errInvalidLoggingType = errors.New("invalid logging type")
	// errInvalidLoggingLevel is returned when an invalid logging level is provided.
	errInvalidLoggingLevel = errors.New("invalid logging level")
)

// Logging contains configuration for logging.
type Logging struct {
	// Type indicates where logs should be written, defaulting to "stdout".
	Type string `mapstructure:"type" yaml:"type,omitempty"`

	// Level is the log level to use, defaulting to "info".
	Level LogLevel `mapstructure:"level" yaml:"level,omitempty"`
}

// Validate validates the logging configuration.
func (l *Logging) Validate() error {
	// Type is optional; empty means use default. If set, must be stdout.
	switch strings.ToLower(strings.TrimSpace(l.Type)) {
	case "":
		// allow empty, defaults applied elsewhere (overrides)
	case LoggingTypeStdout:
		// ok
	default:
		return fmt.Errorf("%w: %s", errInvalidLoggingType, l.Type)
	}

	switch strings.ToLower(string(l.Level)) {
	case "":
		// allow empty, defaults applied elsewhere (overrides)
	case string(LogLevelDebug), string(LogLevelInfo), string(LogLevelWarn), string(LogLevelError):
		// ok
	default:
		return fmt.Errorf("%w: %s", errInvalidLoggingLevel, l.Level)
	}

	return nil
}
