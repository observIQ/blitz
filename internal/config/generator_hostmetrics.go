package config

import (
	"fmt"
	"time"
)

// HostMetricsGeneratorConfig contains configuration for host metrics generator
type HostMetricsGeneratorConfig struct {
	// Workers is the number of worker goroutines for host metrics generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the scrape interval for host metrics
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// OS is the simulated operating system. One of: linux, windows
	OS string `yaml:"os,omitempty" mapstructure:"os,omitempty"`
	// Hostname is the simulated hostname. If empty, a random hostname is generated.
	Hostname string `yaml:"hostname,omitempty" mapstructure:"hostname,omitempty"`
	// Scrapers is the list of scrapers to enable. If empty, all scrapers are enabled.
	Scrapers []string `yaml:"scrapers,omitempty" mapstructure:"scrapers,omitempty"`
}

// ValidScrapers is the list of valid scraper names.
var ValidScrapers = []string{
	"cpu", "memory", "disk", "network", "filesystem", "load", "paging", "processes",
}

// Validate validates the host metrics generator configuration
func (c *HostMetricsGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("hostmetrics generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("hostmetrics generator rate must be positive, got %v", c.Rate)
	}

	if c.OS != "" && c.OS != "linux" && c.OS != "windows" {
		return fmt.Errorf("hostmetrics generator OS must be one of: linux, windows, got %q", c.OS)
	}

	for _, s := range c.Scrapers {
		if !isValidScraper(s) {
			return fmt.Errorf("hostmetrics generator invalid scraper %q, must be one of: %v", s, ValidScrapers)
		}
	}

	return nil
}

func isValidScraper(name string) bool {
	for _, v := range ValidScrapers {
		if v == name {
			return true
		}
	}
	return false
}
