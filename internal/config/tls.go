package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

const (
	// DefaultMinTLSVersion is the default minimum TLS version
	// used when creating a TLS config
	DefaultMinTLSVersion uint16 = tls.VersionTLS13
)

// TLSVersion is the TLS version configured by the user
type TLSVersion string

const (
	// TLSVersion12 represents TLS 1.2
	TLSVersion12 TLSVersion = "1.2"

	// TLSVersion13 represents TLS 1.3
	TLSVersion13 TLSVersion = "1.3"
)

func (t TLSVersion) parseTLSVersion() uint16 {
	switch t {
	case "1.2":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	default:
		return DefaultMinTLSVersion
	}
}

// TLS is the configuration for TLS connections
type TLS struct {
	// The minimum TLS version that is acceptable for incoming connections.
	MinTLSVersion TLSVersion `mapstructure:"tlsMinVersion" yaml:"tlsMinVersion,omitempty"`

	// Certificate is the path to the x509 PEM encoded certificate file that will be used to
	// establish TLS connections.
	//
	// When operating in server mode, this certificate is presented to clients.
	// When operating in client mode with mTLS, this certificate is used for authentication
	// against the server.
	Certificate string `mapstructure:"tlsCert" yaml:"tlsCert,omitempty"`

	// PrivateKey is the matching x509 PEM encoded private key for the Certificate.
	PrivateKey string `mapstructure:"tlsKey" yaml:"tlsKey,omitempty"`

	// CertificateAuthority is one or more file paths to x509 PEM encoded certificate authority chains.
	// These certificate authorities are used for trusting incoming client mTLS connections.
	CertificateAuthority []string `mapstructure:"tlsCa" yaml:"tlsCa,omitempty"`

	// InsecureSkipVerify controls whether a client verifies the server's certificate chain and host name. If
	// InsecureSkipVerify is true, crypto/tls accepts any certificate presented by the server and any host name in that
	// certificate.
	//
	// It is also used to signal that clients, like the agent metrics pipeline, should connect to the server with
	// tls.insecure set to true.
	//
	// In this mode, TLS is susceptible to machine-in-the-middle attacks. This should be used only for testing only.
	InsecureSkipVerify bool `mapstructure:"tlsSkipVerify" yaml:"tlsSkipVerify,omitempty"`
}

// TLSEnabled returns true if TLS is configured
func (t *TLS) TLSEnabled() bool {
	return t.Certificate != "" && t.PrivateKey != ""
}

// Validate validates the TLS configuration
func (t *TLS) Validate() error {
	switch t.MinTLSVersion {
	// Zero value will cause fallback to DefaultMinTLSVersion
	case "", TLSVersion12, TLSVersion13:
	default:
		return fmt.Errorf(
			"invalid tls version %s, should be one of %s, %s",
			t.MinTLSVersion, TLSVersion12, TLSVersion13,
		)
	}

	if t.Certificate != "" && t.PrivateKey == "" {
		return errors.New("tls private key must be set when tls certificate is set")
	}

	if t.Certificate == "" && t.PrivateKey != "" {
		return errors.New("tls certificate must be set when tls private key is set")
	}

	if t.Certificate != "" {
		if _, err := os.Stat(t.Certificate); err != nil {
			return fmt.Errorf("failed to lookup tls certificate file %s: %w", t.Certificate, err)
		}
	}

	if t.PrivateKey != "" {
		if _, err := os.Stat(t.PrivateKey); err != nil {
			return fmt.Errorf("failed to lookup tls private key file %s: %w", t.PrivateKey, err)
		}
	}

	if len(t.CertificateAuthority) > 0 {
		for _, ca := range t.CertificateAuthority {
			if _, err := os.Stat(ca); err != nil {
				return fmt.Errorf("failed to lookup tls certificate authority file %s: %w", ca, err)
			}
		}
	}

	return nil
}

// Convert converts a TLS config to a *tls.Config
func (t TLS) Convert() (*tls.Config, error) {
	minTLS := t.MinTLSVersion.parseTLSVersion()

	// #nosec G402 - User defines min tls version via a flag, user is restricted to 1.2 or 1.3
	tlsConfig := &tls.Config{
		MinVersion:         minTLS,
		InsecureSkipVerify: false,
	}

	if t.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = t.InsecureSkipVerify
	}

	// CA certificate can be used to trust private certificates
	if len(t.CertificateAuthority) > 0 {
		var caPool = x509.NewCertPool()

		for _, caCertFile := range t.CertificateAuthority {
			ca, err := os.ReadFile(caCertFile) // #nosec G304, user defines ca file path via a flag
			if err != nil {
				return nil, fmt.Errorf("failed to read certificate authority file: %w", err)
			}

			if !caPool.AppendCertsFromPEM(ca) {
				return nil, errors.New("failed to append certificate authority to root ca pool")
			}
		}

		// Require mTLS on the server if CAs are specified
		tlsConfig.ClientCAs = caPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

		// For clients, caPool will be used for verifying server certificates.
		tlsConfig.RootCAs = caPool
	}

	// Client key pair can be used for mutual TLS
	if t.Certificate != "" && t.PrivateKey != "" {
		keypair, err := tls.LoadX509KeyPair(t.Certificate, t.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load tls certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{keypair}
	}

	return tlsConfig, nil
}
