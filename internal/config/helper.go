package config

import (
	"fmt"
	"net"
	"regexp"
)

// ValidatePort validates that a port number is valid
func ValidatePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}

	return nil
}

// hostnameRegex is the regex pattern for validating hostnames
// This regex validates hostnames according to RFC 1123 standards
var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)

// ValidateHost validates that a host string is a valid IP address or hostname
func ValidateHost(host string) error {
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// Check if it's a valid IP address first
	if net.ParseIP(host) != nil {
		return nil
	}

	// Check hostname length (max 253 characters)
	if len(host) > 253 {
		return fmt.Errorf("host must be a valid IP address or hostname")
	}

	// If it's not a valid IP, check if it's a valid hostname using regex
	if !hostnameRegex.MatchString(host) {
		return fmt.Errorf("host must be a valid IP address or hostname")
	}

	return nil
}
