package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/unpackdev/fdb/types"
	"gopkg.in/yaml.v3"
	"os"
)

// TcpTransport represents the configuration for TCP-based transport.
// It implements the TransportConfig interface and provides all necessary fields
// to configure and use TCP transport.
type TcpTransport struct {
	// Type defines the transport type, typically represented as types.TCPTransportType.
	Type types.TransportType `yaml:"type" json:"type" mapstructure:"type"`

	// Enabled determines if the TCP transport is enabled.
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`

	// IPv4 is the IPv4 address where the TCP server or client will bind.
	IPv4 string `yaml:"ipv4" json:"ipv4" mapstructure:"ipv4"`

	// Port is the port number on which the TCP transport will operate.
	Port int `yaml:"port" json:"port" mapstructure:"port"`

	// TLS holds the TLS configuration for the TCP transport, if TLS is required.
	TLS *TLS `yaml:"tls" json:"tls" mapstructure:"tls"`
}

// Addr returns the full address (IPv4 and port) as a string for the TCP transport.
// This address is used by the TCP server or client to bind or connect to.
//
// Example usage:
//
//	addr := tcpTransport.Addr()
//
// Returns:
//
//	string: The full IPv4 address and port.
func (t TcpTransport) Addr() string {
	return fmt.Sprintf("%s:%d", t.IPv4, t.Port)
}

// GetTransportType returns the transport type, which is typically TCP for this struct.
//
// Example usage:
//
//	transportType := tcpTransport.GetTransportType()
//
// Returns:
//
//	types.TransportType: The transport type.
func (t TcpTransport) GetTransportType() types.TransportType {
	return t.Type
}

// GetTLSConfig loads the TLS configuration if specified. This allows the TCP transport
// to use TLS for secure communication.
//
// Example usage:
//
//	tlsConfig, err := tcpTransport.GetTLSConfig()
//	if err != nil {
//	    log.Fatalf("Failed to load TLS config: %v", err)
//	}
//
// Returns:
//
//	*tls.Config: The TLS configuration for the TCP transport, or nil if not using TLS.
//	error: Returns an error if TLS setup fails.
func (t TcpTransport) GetTLSConfig() (*tls.Config, error) {
	if t.TLS == nil {
		return nil, nil // No TLS configuration provided
	}

	// Check if the certificate file exists
	if _, err := os.Stat(t.TLS.Cert); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate file does not exist: %s", t.TLS.Cert)
	}
	// Check if the key file exists
	if _, err := os.Stat(t.TLS.Key); os.IsNotExist(err) {
		return nil, fmt.Errorf("key file does not exist: %s", t.TLS.Key)
	}

	// Load the certificate and key
	cert, err := tls.LoadX509KeyPair(t.TLS.Cert, t.TLS.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate and key: %w", err)
	}

	// Prepare the TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: t.TLS.Insecure,
		Certificates:       []tls.Certificate{cert},
	}

	// Load the Root CA if specified
	if t.TLS.RootCA != "" {
		caCert, err := os.ReadFile(t.TLS.RootCA)
		if err != nil {
			return nil, fmt.Errorf("failed to read root CA file: %w", err)
		}

		// Append the Root CA to the pool
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append root CA certificates")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// UnmarshalYAML is a custom YAML unmarshaler for TcpTransport.
// It decodes the YAML configuration into the TcpTransport struct fields,
// mapping the common transport fields like Type, Enabled, IPv4, Port, and TLS.
//
// Example YAML format:
//
//		type: tcp
//		enabled: true
//		ipv4: "127.0.0.1"
//		port: 4242
//		tls:
//	      insecure: true
//		  cert: "/path/to/cert.pem"
//		  key: "/path/to/key.pem"
//		  rootCa: "/path/to/rootCA.pem"
//
// Parameters:
//
//	value (*yaml.Node): The YAML node to be decoded.
//
// Returns:
//
//	error: Returns an error if unmarshaling fails; otherwise, nil.
func (t *TcpTransport) UnmarshalYAML(value *yaml.Node) error {
	// Create a temporary struct to capture the common fields
	aux := struct {
		Type    types.TransportType `yaml:"type"`
		Enabled bool                `yaml:"enabled"`
		IPv4    string              `yaml:"ipv4"`
		Port    int                 `yaml:"port"`
		TLS     *TLS                `yaml:"tls"`
	}{}

	// Unmarshal the common fields, including the nested TLS config
	if err := value.Decode(&aux); err != nil {
		return fmt.Errorf("failed to unmarshal TCP transport fields: %w", err)
	}

	// Assign values to the actual struct
	t.Type = aux.Type
	t.Enabled = aux.Enabled
	t.IPv4 = aux.IPv4
	t.Port = aux.Port
	t.TLS = aux.TLS

	return nil
}
