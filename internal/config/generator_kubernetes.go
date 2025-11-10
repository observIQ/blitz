package config

import (
	"fmt"
	"time"
)

const (
	// KubernetesFormatCRIO is the CRI-O container log format
	KubernetesFormatCRIO = "cri-o"
)

// KubernetesGeneratorConfig contains configuration for Kubernetes container log generator
type KubernetesGeneratorConfig struct {
	// Workers is the number of worker goroutines for Kubernetes generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// Format is the container log format. Valid values: "cri-o"
	Format string `yaml:"format,omitempty" mapstructure:"format,omitempty"`
}

// Validate validates the Kubernetes generator configuration
func (c *KubernetesGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("Kubernetes generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("Kubernetes generator rate must be positive, got %v", c.Rate)
	}

	switch c.Format {
	case "", KubernetesFormatCRIO:
		// Valid format, no error
	default:
		return fmt.Errorf("Kubernetes generator format must be one of: %s, got %q", KubernetesFormatCRIO, c.Format)
	}

	return nil
}
