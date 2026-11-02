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
	// Mode is how lines are picked: random (the default) picks a random line
	// on each emission; replay emits every non-empty line once per pass, in
	// order (see docs/generator/filegen.md for restarts).
	Mode string `yaml:"mode,omitempty" mapstructure:"mode,omitempty"`
	// Seed seeds the random mode's line picks per the contract in
	// docs/datagen.md: negative randomizes (the resolved seed is logged), 0
	// or positive is deterministic (worker N uses Seed+N), and an omitted
	// key defaults to -1. Replay ignores it.
	Seed int64 `yaml:"seed,omitempty" mapstructure:"seed,omitempty"`
}

// File generator modes.
const (
	FileModeRandom = "random"
	FileModeReplay = "replay"
)

// IsLegacyFileMode reports whether mode is one of the mode key's old
// source-type values (file, directory, package). The source type is now
// detected, so a config that still sets one runs random, the default.
func IsLegacyFileMode(mode string) bool {
	switch mode {
	case "file", "directory", "package":
		return true
	}
	return false
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

	switch {
	case c.Mode == "", c.Mode == FileModeRandom, IsLegacyFileMode(c.Mode):
	case c.Mode == FileModeReplay:
		if c.Workers != 1 {
			return fmt.Errorf("File generator replay mode requires exactly 1 worker, got %d", c.Workers)
		}
	default:
		return fmt.Errorf("File generator mode must be one of random|replay, got %q", c.Mode)
	}

	return nil
}
