package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/unpackdev/fdb/types"
	"gopkg.in/yaml.v3"
	"os"
)

// QuicTransport represents the configuration for QUIC-based transport.
// It implements the TransportConfig interface and provides all necessary fields
// to configure and use QUIC transport, which is a fast, connection-oriented protocol
// over UDP, often used for low-latency applications.
type QuicTransport struct {
	// Type defines the transport type, typically represented as types.QUICTransportType.
	Type types.TransportType `yaml:"type" json:"type" mapstructure:"type"`

	// Enabled determines if the QUIC transport is enabled.
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`

	// IPv4 is the IPv4 address where the QUIC server or client will bind.
	IPv4 string `yaml:"ipv4" json:"ipv4" mapstructure:"ipv4"`

	// Port is the port number on which the QUIC transport will operate.
	Port int `yaml:"port" json:"port" mapstructure:"port"`

	// TLS holds the TLS configuration for the QUIC transport, as QUIC requires
	// TLS for secure communication.
	TLS TLS `yaml:"tls" json:"tls" mapstructure:"tls"`
}

// Addr returns the full address (IPv4 and port) as a string for the QUIC transport.
// This address is used by the QUIC server or client to bind or connect to.
//
// Example usage:
//
//	addr := quicTransport.Addr()
//
// Returns:
//
//	string: The full IPv4 address and port.
func (q QuicTransport) Addr() string {
	return fmt.Sprintf("%s:%d", q.IPv4, q.Port)
}

// GetTransportType returns the transport type, which is typically QUIC for this struct.
//
// Example usage:
//
//	transportType := quicTransport.GetTransportType()
//
// Returns:
//
//	types.TransportType: The transport type.
func (q QuicTransport) GetTransportType() types.TransportType {
	return q.Type
}

// GetTLSConfig loads the TLS configuration required for QUIC transport.
// It checks for the existence of the certificate and key files, loads them,
// and optionally loads the Root CA if specified.
//
// Example usage:
//
//	tlsConfig, err := quicTransport.GetTLSConfig()
//	if err != nil {
//	    log.Fatalf("Failed to load TLS config: %v", err)
//	}
//
// Returns:
//
//	*tls.Config: The TLS configuration for the QUIC transport.
//	error: Returns an error if TLS setup fails.
func (q QuicTransport) GetTLSConfig() (*tls.Config, error) {
	// Check if the certificate file exists
	if _, err := os.Stat(q.TLS.Cert); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate file does not exist: %s", q.TLS.Cert)
	}
	// Check if the key file exists
	if _, err := os.Stat(q.TLS.Key); os.IsNotExist(err) {
		return nil, fmt.Errorf("key file does not exist: %s", q.TLS.Key)
	}

	// Load the certificate and key
	cert, err := tls.LoadX509KeyPair(q.TLS.Cert, q.TLS.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate and key: %w", err)
	}

	// Prepare the TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"quic-example"}, // Required for QUIC, ALPN must match between client and server
	}

	// Load the Root CA if specified
	if q.TLS.RootCA != "" {
		caCert, err := os.ReadFile(q.TLS.RootCA)
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

// UnmarshalYAML is a custom YAML unmarshaler for QuicTransport.
// It decodes the YAML configuration into the QuicTransport struct fields,
// mapping the common transport fields like Type, Enabled, IPv4, Port, and TLS.
//
// Example YAML format:
//
//	type: quic
//	enabled: true
//	ipv4: "127.0.0.1"
//	port: 4242
//	tls:
//	  cert: "/path/to/cert.pem"
//	  key: "/path/to/key.pem"
//	  rootCa: "/path/to/rootCA.pem"
//
// Parameters:
//
//	value (*yaml.Node): The YAML node to be decoded.
//
// Returns:
//
//	error: Returns an error if unmarshaling fails; otherwise, nil.
func (q *QuicTransport) UnmarshalYAML(value *yaml.Node) error {
	// Create a temporary struct to capture the common fields
	aux := struct {
		Type    types.TransportType `yaml:"type"`
		Enabled bool                `yaml:"enabled"`
		IPv4    string              `yaml:"ipv4"`
		Port    int                 `yaml:"port"`
		TLS     TLS                 `yaml:"tls"`
	}{}

	// Unmarshal the common fields, including the nested TLS config
	if err := value.Decode(&aux); err != nil {
		return fmt.Errorf("failed to unmarshal QUIC transport fields: %w", err)
	}

	// Assign values to the actual struct
	q.Type = aux.Type
	q.Enabled = aux.Enabled
	q.IPv4 = aux.IPv4
	q.Port = aux.Port
	q.TLS = aux.TLS

	return nil
}
