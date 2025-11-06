package config

import (
	"fmt"
	"strings"
)

// SyslogRFC selects the syslog message format.
type SyslogRFC string

const (
	// SyslogRFC3164 formats messages as RFC 3164.
	SyslogRFC3164 SyslogRFC = "3164"
	// SyslogRFC5424 formats messages as RFC 5424.
	SyslogRFC5424 SyslogRFC = "5424"
)

// SyslogTransport selects the syslog transport.
type SyslogTransport string

const (
	// SyslogTransportTCP uses TCP transport.
	SyslogTransportTCP SyslogTransport = "tcp"
	// SyslogTransportUDP uses UDP transport.
	SyslogTransportUDP SyslogTransport = "udp"
)

// SyslogOutputConfig contains configuration for Syslog output (wrapping TCP/UDP).
type SyslogOutputConfig struct {
	Host      string          `yaml:"host,omitempty" mapstructure:"host,omitempty"`
	Port      int             `yaml:"port,omitempty" mapstructure:"port,omitempty"`
	Transport SyslogTransport `yaml:"transport,omitempty" mapstructure:"transport,omitempty"`
	RFC       SyslogRFC       `yaml:"rfc,omitempty" mapstructure:"rfc,omitempty"`
	Workers   int             `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`

	Facility int    `yaml:"facility,omitempty" mapstructure:"facility,omitempty"`
	AppName  string `yaml:"appName,omitempty" mapstructure:"appName,omitempty"`
	Hostname string `yaml:"hostname,omitempty" mapstructure:"hostname,omitempty"`
	ProcID   string `yaml:"procId,omitempty" mapstructure:"procId,omitempty"`
	MsgID    string `yaml:"msgId,omitempty" mapstructure:"msgId,omitempty"`

	// UDP-only safety limit
	MaxDatagramBytes int `yaml:"maxDatagramBytes,omitempty" mapstructure:"maxDatagramBytes,omitempty"`

	// TCP-only TLS
	EnableTLS bool `yaml:"enableTLS,omitempty" mapstructure:"enableTLS,omitempty"`
	TLS       `yaml:",inline"`
}

// Validate validates the syslog output configuration.
func (c *SyslogOutputConfig) Validate() error {
	if err := ValidateHost(c.Host); err != nil {
		return fmt.Errorf("Syslog output host validation failed: %w", err)
	}
	if err := ValidatePort(c.Port); err != nil {
		return fmt.Errorf("Syslog output port validation failed: %w", err)
	}
	if c.Workers < 0 {
		return fmt.Errorf("Syslog output workers cannot be negative, got %d", c.Workers)
	}
	switch strings.ToLower(string(c.Transport)) {
	case "", string(SyslogTransportUDP), string(SyslogTransportTCP):
	default:
		return fmt.Errorf("Syslog output transport must be one of: tcp|udp")
	}
	switch c.RFC {
	case "", SyslogRFC3164, SyslogRFC5424:
	default:
		return fmt.Errorf("Syslog output rfc must be one of: 3164|5424")
	}
	if c.Facility < 0 || c.Facility > 23 {
		return fmt.Errorf("Syslog output facility must be between 0 and 23, got %d", c.Facility)
	}
	if strings.ToLower(string(c.Transport)) == string(SyslogTransportTCP) && c.EnableTLS {
		if err := c.TLS.Validate(); err != nil {
			return fmt.Errorf("Syslog output TLS validation failed: %w", err)
		}
	}
	return nil
}
