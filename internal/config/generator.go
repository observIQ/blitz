package config

import (
	"fmt"
)

// GeneratorType represents the type of generator
type GeneratorType string

const (
	// GeneratorTypeNop represents NOP generator
	GeneratorTypeNop GeneratorType = "nop"
	// GeneratorTypeJSON represents JSON generator
	GeneratorTypeJSON GeneratorType = "json"
	// GeneratorTypeWinevt represents Windows Event XML generator
	GeneratorTypeWinevt GeneratorType = "winevt"
	// GeneratorTypePaloAlto represents Palo Alto syslog generator
	GeneratorTypePaloAlto GeneratorType = "palo-alto"
	// GeneratorTypeApache represents Apache Common Log Format generator
	GeneratorTypeApache GeneratorType = "apache-common"
	// GeneratorTypeApacheCombined represents Apache Combined Log Format generator
	GeneratorTypeApacheCombined GeneratorType = "apache-combined"
	// GeneratorTypeApacheError represents Apache Error Log Format generator
	GeneratorTypeApacheError GeneratorType = "apache-error"
	// GeneratorTypeNginx represents NGINX Combined Log Format generator
	GeneratorTypeNginx GeneratorType = "nginx"
	// GeneratorTypePostgres represents PostgreSQL log format generator
	GeneratorTypePostgres GeneratorType = "postgres"
	// GeneratorTypeKubernetes represents Kubernetes container log format generator
	GeneratorTypeKubernetes GeneratorType = "kubernetes"
	// GeneratorTypeFile represents File generator
	GeneratorTypeFile GeneratorType = "filegen"
)

// Generator contains configuration for log generators
type Generator struct {
	// Type specifies the generator type (json)
	Type GeneratorType `yaml:"type,omitempty" mapstructure:"type,omitempty"`
	// JSON contains JSON generator configuration
	JSON JSONGeneratorConfig `yaml:"json,omitempty" mapstructure:"json,omitempty"`
	// Winevt contains Windows Event generator configuration
	Winevt WinevtGeneratorConfig `yaml:"winevt,omitempty" mapstructure:"winevt,omitempty"`
	// PaloAlto contains Palo Alto generator configuration
	PaloAlto PaloAltoGeneratorConfig `yaml:"paloAlto,omitempty" mapstructure:"paloAlto,omitempty"`
	// Apache contains Apache generator configuration
	Apache ApacheGeneratorConfig `yaml:"apache-common,omitempty" mapstructure:"apache-common,omitempty"`
	// ApacheCombined contains Apache Combined generator configuration
	ApacheCombined ApacheCombinedGeneratorConfig `yaml:"apache-combined,omitempty" mapstructure:"apache-combined,omitempty"`
	// ApacheError contains Apache Error generator configuration
	ApacheError ApacheErrorGeneratorConfig `yaml:"apache-error,omitempty" mapstructure:"apache-error,omitempty"`
	// Nginx contains NGINX generator configuration
	Nginx NginxGeneratorConfig `yaml:"nginx,omitempty" mapstructure:"nginx,omitempty"`
	// Postgres contains PostgreSQL generator configuration
	Postgres PostgresGeneratorConfig `yaml:"postgres,omitempty" mapstructure:"postgres,omitempty"`
	// Kubernetes contains Kubernetes container log generator configuration
	Kubernetes KubernetesGeneratorConfig `yaml:"kubernetes,omitempty" mapstructure:"kubernetes,omitempty"`
	// Filegen contains File generator configuration
	Filegen FileGeneratorConfig `yaml:"filegen,omitempty" mapstructure:"filegen,omitempty"`
}

// Validate validates the generator configuration
func (g *Generator) Validate() error {
	// Allow empty type - defaults will be applied by override system
	if g.Type == "" {
		return nil
	}

	switch g.Type {
	case GeneratorTypeNop:
		// NOP generator requires no additional validation
	case GeneratorTypeJSON:
		if err := g.JSON.Validate(); err != nil {
			return fmt.Errorf("JSON generator validation failed: %w", err)
		}
	case GeneratorTypeWinevt:
		if err := g.Winevt.Validate(); err != nil {
			return fmt.Errorf("winevt generator validation failed: %w", err)
		}
	case GeneratorTypePaloAlto:
		if err := g.PaloAlto.Validate(); err != nil {
			return fmt.Errorf("palo-alto generator validation failed: %w", err)
		}
	case GeneratorTypeApache:
		if err := g.Apache.Validate(); err != nil {
			return fmt.Errorf("apache generator validation failed: %w", err)
		}
	case GeneratorTypeApacheCombined:
		if err := g.ApacheCombined.Validate(); err != nil {
			return fmt.Errorf("apache-combined generator validation failed: %w", err)
		}
	case GeneratorTypeApacheError:
		if err := g.ApacheError.Validate(); err != nil {
			return fmt.Errorf("apache-error generator validation failed: %w", err)
		}
	case GeneratorTypeNginx:
		if err := g.Nginx.Validate(); err != nil {
			return fmt.Errorf("nginx generator validation failed: %w", err)
		}
	case GeneratorTypePostgres:
		if err := g.Postgres.Validate(); err != nil {
			return fmt.Errorf("postgres generator validation failed: %w", err)
		}
	case GeneratorTypeKubernetes:
		if err := g.Kubernetes.Validate(); err != nil {
			return fmt.Errorf("kubernetes generator validation failed: %w", err)
		}
	case GeneratorTypeFile:
		if err := g.Filegen.Validate(); err != nil {
			return fmt.Errorf("filegen generator validation failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid generator type: %s, must be one of: nop, json, winevt, palo-alto, apache-common, apache-combined, apache-error, nginx, postgres, kubernetes, filegen", g.Type)
	}

	return nil
}
