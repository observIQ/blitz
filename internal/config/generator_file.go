package config

import (
	"fmt"
	"time"
)

// FileGeneratorConfig contains configuration for File log generator
type FileGeneratorConfig struct {
	// Workers is the number of worker goroutines for file reading
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are written per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// Source is the file path, directory path, or glob pattern (auto-detected)
	Source string `yaml:"source,omitempty" mapstructure:"source,omitempty"`
	// CacheEnabled enables in-memory file caching (default true)
	CacheEnabled bool `yaml:"cache-enabled,omitempty" mapstructure:"cache-enabled,omitempty"`
	// CacheTTL is the time-to-live for cached file entries (0 = never expire, default = 0)
	CacheTTL time.Duration `yaml:"cache-ttl,omitempty" mapstructure:"cache-ttl,omitempty"`
}

// Validate validates the File generator configuration
func (c *FileGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("File generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("File generator rate must be positive, got %v", c.Rate)
	}

	if c.Source == "" {
		return fmt.Errorf("File generator source cannot be empty")
	}

	return nil
}
