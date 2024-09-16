package config

import (
	"crypto/tls"
	"fmt"
	"github.com/unpackdev/fdb/types"
	"gopkg.in/yaml.v3"
)

// DummyTransport represents the configuration for a dummy transport used in testing or
// development environments. It implements the TransportConfig interface, providing basic
// transport settings like IP address, port, and optional TLS configurations.
type DummyTransport struct {
	// Type specifies the transport type, typically represented as types.DummyTransportType.
	Type types.TransportType `yaml:"type" json:"type" mapstructure:"type"`

	// Enabled determines whether this transport configuration is active.
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`

	// IPv4 defines the IPv4 address the dummy transport will bind to.
	IPv4 string `yaml:"ipv4" json:"ipv4" mapstructure:"ipv4"`

	// Port specifies the port on which the dummy transport will listen.
	Port int `yaml:"port" json:"port" mapstructure:"port"`

	// TLS holds the TLS configuration for the dummy transport.
	// Although this is typically not used in dummy transports, it allows for optional secure communication.
	TLS TLS `yaml:"tls" json:"tls" mapstructure:"tls"`
}

// Addr returns the full address (IPv4 and port) as a string for the dummy transport.
// This is the address the dummy transport binds to.
//
// Example usage:
//
//	addr := dummyTransport.Addr()
//
// Returns:
//
//	string: The formatted IPv4 address and port.
func (q DummyTransport) Addr() string {
	return fmt.Sprintf("%s:%d", q.IPv4, q.Port)
}

// GetTransportType returns the transport type, which is typically Dummy for this struct.
//
// Example usage:
//
//	transportType := dummyTransport.GetTransportType()
//
// Returns:
//
//	types.TransportType: The transport type for the dummy transport.
func (q DummyTransport) GetTransportType() types.TransportType {
	return q.Type
}

// GetTLSConfig returns a basic TLS configuration for the dummy transport. It skips certificate verification
// and uses an empty list of certificates by default, making it more suitable for testing or internal use.
//
// Example usage:
//
//	tlsConfig, err := dummyTransport.GetTLSConfig()
//	if err != nil {
//	    log.Fatalf("Failed to get TLS config: %v", err)
//	}
//
// Returns:
//
//	*tls.Config: The TLS configuration for the dummy transport.
//	error: Returns an error if setting up TLS fails.
func (q DummyTransport) GetTLSConfig() (*tls.Config, error) {
	return &tls.Config{
		InsecureSkipVerify: true, // Skip verification for testing purposes
		Certificates:       []tls.Certificate{},
	}, nil
}

// UnmarshalYAML provides custom unmarshaling logic for the DummyTransport from a YAML configuration.
// It decodes the common fields such as type, enabled status, IPv4 address, port, and TLS configuration.
//
// Example YAML format:
//
//	type: dummy
//	enabled: true
//	ipv4: "127.0.0.1"
//	port: 8080
//	tls:
//	  insecure: true
//
// Parameters:
//
//	value (*yaml.Node): The YAML node to be decoded.
//
// Returns:
//
//	error: Returns an error if the unmarshaling fails; otherwise, nil.
func (d *DummyTransport) UnmarshalYAML(value *yaml.Node) error {
	aux := struct {
		Type    types.TransportType `yaml:"type"`
		Enabled bool                `yaml:"enabled"`
		IPv4    string              `yaml:"ipv4"`
		Port    int                 `yaml:"port"`
		TLS     TLS                 `yaml:"tls"`
	}{}

	if err := value.Decode(&aux); err != nil {
		return fmt.Errorf("failed to unmarshal dummy transport fields: %w", err)
	}

	// Assign values to the actual struct
	d.Type = aux.Type
	d.Enabled = aux.Enabled
	d.IPv4 = aux.IPv4
	d.Port = aux.Port
	d.TLS = aux.TLS

	return nil
}
