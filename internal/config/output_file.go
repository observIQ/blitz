package config

import "fmt"

// Default values for File output rotation
const (
	// DefaultFileWorkers is the default number of worker goroutines for file output
	DefaultFileWorkers = 1

	// DefaultFileRotationMaxSizeMB is the default max size in megabytes before rotation
	DefaultFileRotationMaxSizeMB = 100

	// DefaultFileRotationMaxBackups is the default number of old log files to retain
	DefaultFileRotationMaxBackups = 7

	// DefaultFileRotationMaxAgeDays is the default max number of days to retain old log files
	DefaultFileRotationMaxAgeDays = 30
)

// FileRotationConfig contains rotation options for file output
type FileRotationConfig struct {
	// MaxSizeMB is the maximum size in megabytes before the log is rotated
	MaxSizeMB int `yaml:"maxSizeMB,omitempty" mapstructure:"maxSizeMB,omitempty"`

	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int `yaml:"maxBackups,omitempty" mapstructure:"maxBackups,omitempty"`

	// MaxAgeDays is the maximum number of days to retain old log files
	MaxAgeDays int `yaml:"maxAgeDays,omitempty" mapstructure:"maxAgeDays,omitempty"`

	// Compress determines if the rotated log files should be compressed
	Compress bool `yaml:"compress,omitempty" mapstructure:"compress,omitempty"`

	// LocalTime determines if the time used for formatting the backup filename is the computer's local time
	LocalTime bool `yaml:"localTime,omitempty" mapstructure:"localTime,omitempty"`
}

// FileOutputConfig contains configuration for file output
type FileOutputConfig struct {
	// Path is the destination file path
	Path string `yaml:"path,omitempty" mapstructure:"path,omitempty"`

	// Workers controls the number of concurrent writers
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`

	// Rotation contains rotation options
	Rotation FileRotationConfig `yaml:"rotation,omitempty" mapstructure:"rotation,omitempty"`
}

// Validate validates the file output configuration
func (f *FileOutputConfig) Validate() error {
	if f.Path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	return nil
}
