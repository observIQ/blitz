package config

import (
	"fmt"
	"net"
)

// ValidatePort validates that a port number is valid
func ValidatePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}

	return nil
}

// ValidateHost validates that a host string is a valid IP address or hostname
func ValidateHost(host string) error {
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// Check if it's a valid IP address first
	if net.ParseIP(host) != nil {
		return nil
	}

	// If it's not a valid IP, check if it's a valid hostname
	if _, err := net.LookupHost(host); err != nil {
		return fmt.Errorf("host must be a valid IP address or hostname: %w", err)
	}

	return nil
}
