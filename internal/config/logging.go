package config

import (
	"errors"
	"fmt"
	"strings"
)

// LogLevel represents a supported logging severity level.
type LogLevel string

const (
	// LoggingTypeStdout writes logs to stdout.
	LoggingTypeStdout = "stdout"
	// LoggingTypeFile writes logs to a file with rotation.
	LoggingTypeFile = "file"

	// LogLevelDebug is the debug log level.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo is the info log level.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn is the warn log level.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError is the error log level.
	LogLevelError LogLevel = "error"
)

// Default logging file path
const (
	// DefaultLoggingFilePath is the default path for file logging
	DefaultLoggingFilePath = "/var/log/blitz/blitz.log"
)

var (
	// errInvalidLoggingType is returned when an invalid logging type is provided.
	errInvalidLoggingType = errors.New("invalid logging type")
	// errInvalidLoggingLevel is returned when an invalid logging level is provided.
	errInvalidLoggingLevel = errors.New("invalid logging level")
)

// Logging contains configuration for logging.
type Logging struct {
	// Type indicates where logs should be written.
	Type string `mapstructure:"type" yaml:"type,omitempty"`

	// Level is the log level to use, defaulting to "info".
	Level LogLevel `mapstructure:"level" yaml:"level,omitempty"`

	// File contains file logging configuration (only used when Type is "file").
	File LoggingFileConfig `mapstructure:"file,omitempty" yaml:"file,omitempty"`
}

// LoggingFileConfig contains configuration for file logging.
type LoggingFileConfig struct {
	// Path is the destination file path for logs.
	Path string `mapstructure:"path,omitempty" yaml:"path,omitempty"`

	// Rotation contains rotation options for log files.
	Rotation FileRotationConfig `mapstructure:"rotation,omitempty" yaml:"rotation,omitempty"`
}

// Validate validates the logging configuration.
func (l *Logging) Validate() error {
	// Type must be set. Config overrides will ensure it's set at runtime.
	trimmedType := strings.ToLower(strings.TrimSpace(l.Type))
	if trimmedType == "" {
		return fmt.Errorf("%w: logging.type is required", errInvalidLoggingType)
	}

	switch trimmedType {
	case LoggingTypeStdout:
		// ok
	case LoggingTypeFile:
		// ok, but path must be set
		if strings.TrimSpace(l.File.Path) == "" {
			return fmt.Errorf("logging.file.path is required when logging.type is file")
		}
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
