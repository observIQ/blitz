package config

import (
	"fmt"
	"time"

	"github.com/observiq/blitz/internal/datagen"
)

// HostMetricsGeneratorConfig contains configuration for host metrics generator
type HostMetricsGeneratorConfig struct {
	// Workers is removed and ignored: one simulated host runs one worker, and
	// Rate is the load knob. Kept only so a still-configured value can be
	// detected and warned about (see LogRemovedSettings).
	//
	// Deprecated: ignored; expected to fail validation as of v0.25.0.
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the scrape interval for host metrics
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// OS is the simulated operating system: any OS datagen.ParseOSType
	// accepts (linux, windows, macos, esxi, xen-dom0, nutanix-ahv,
	// openstack-kvm, aix, solaris, freebsd, openbsd) or one of its aliases.
	// Empty selects linux.
	OS string `yaml:"os,omitempty" mapstructure:"os,omitempty"`
	// Hostname is the simulated hostname. Set, it wins over the environment
	// host and replaces only that host's name. Empty uses the environment
	// host's hostname, or a random hostname when there is no environment host.
	Hostname string `yaml:"hostname,omitempty" mapstructure:"hostname,omitempty"`
	// Scrapers is the list of scrapers to enable. If empty, all scrapers are enabled.
	Scrapers []string `yaml:"scrapers,omitempty" mapstructure:"scrapers,omitempty"`
	// Seed seeds the scrape-value RNG per the contract in docs/datagen.md:
	// negative randomizes (the resolved seed is logged), 0 or positive is
	// deterministic (the RNG uses Seed as given), and an omitted key
	// defaults to -1. Host identity is not governed by this seed: the
	// loaders always build the simulated environment, which picks the host
	// from `environment.seed_config`.
	Seed int64 `yaml:"seed,omitempty" mapstructure:"seed,omitempty"`
}

// ValidScrapers is the list of valid scraper names.
var ValidScrapers = []string{
	"cpu", "memory", "disk", "network", "filesystem", "load", "paging", "processes",
}

// Validate validates the host metrics generator configuration
func (c *HostMetricsGeneratorConfig) Validate() error {
	if c.Rate <= 0 {
		return fmt.Errorf("hostmetrics generator rate must be positive, got %v", c.Rate)
	}

	if c.OS != "" {
		if _, err := datagen.ParseOSType(c.OS); err != nil {
			return fmt.Errorf("hostmetrics generator OS invalid: %w", err)
		}
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
