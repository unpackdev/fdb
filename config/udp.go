package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/unpackdev/fdb/types"
	"gopkg.in/yaml.v3"
	"os"
)

// DTLS represents the DTLS configuration used by the UDP transport.
// It is similar to TLS, but designed for datagram-based communication.
type DTLS struct {
	// Cert is the path to the certificate file used for DTLS encryption.
	Cert string `yaml:"cert" json:"cert" mapstructure:"cert"`

	// Key is the path to the private key file used for DTLS encryption.
	Key string `yaml:"key" json:"key" mapstructure:"key"`

	// RootCA is the path to the root CA file used to validate the peer's certificate.
	RootCA string `yaml:"root_ca" json:"root_ca" mapstructure:"root_ca"`

	// Insecure determines if the DTLS should skip certificate verification.
	Insecure bool `yaml:"insecure" json:"insecure" mapstructure:"insecure"`
}

// UdpTransport represents the configuration for UDP-based transport, with optional DTLS support.
type UdpTransport struct {
	// Type defines the transport type, typically represented as types.UDPTransportType.
	Type types.TransportType `yaml:"type" json:"type" mapstructure:"type"`

	// Enabled determines if the UDP transport is enabled.
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`

	// IPv4 is the IPv4 address where the UDP server or client will bind.
	IPv4 string `yaml:"ipv4" json:"ipv4" mapstructure:"ipv4"`

	// Port is the port number on which the UDP transport will operate.
	Port int `yaml:"port" json:"port" mapstructure:"port"`

	// DTLS holds the DTLS configuration for the UDP transport, if DTLS is required.
	DTLS *DTLS `yaml:"dtls" json:"dtls" mapstructure:"dtls"`
}

// Addr returns the full address (IPv4 and port) as a string for the UDP transport.
// This address is used by the UDP server or client to bind or connect to.
//
// Example usage:
//
//	addr := udpTransport.Addr()
//
// Returns:
//
//	string: The full IPv4 address and port.
func (t UdpTransport) Addr() string {
	return fmt.Sprintf("%s:%d", t.IPv4, t.Port)
}

// GetTransportType returns the transport type, which is typically UDP for this struct.
//
// Example usage:
//
//	transportType := udpTransport.GetTransportType()
//
// Returns:
//
//	types.TransportType: The transport type.
func (t UdpTransport) GetTransportType() types.TransportType {
	return t.Type
}

// GetDTLSConfig loads the DTLS configuration if specified. This allows the UDP transport
// to use DTLS for secure communication.
//
// Example usage:
//
//	dtlsConfig, err := udpTransport.GetDTLSConfig()
//	if err != nil {
//	    log.Fatalf("Failed to load DTLS config: %v", err)
//	}
//
// Returns:
//
//	*tls.Config: The DTLS configuration for the UDP transport, or nil if not using DTLS.
//	error: Returns an error if DTLS setup fails.
func (t UdpTransport) GetDTLSConfig() (*tls.Config, error) {
	if t.DTLS == nil {
		return nil, nil // No DTLS configuration provided
	}

	// Check if the certificate file exists
	if _, err := os.Stat(t.DTLS.Cert); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate file does not exist: %s", t.DTLS.Cert)
	}
	// Check if the key file exists
	if _, err := os.Stat(t.DTLS.Key); os.IsNotExist(err) {
		return nil, fmt.Errorf("key file does not exist: %s", t.DTLS.Key)
	}

	// Load the certificate and key
	cert, err := tls.LoadX509KeyPair(t.DTLS.Cert, t.DTLS.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate and key: %w", err)
	}

	// Prepare the DTLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: t.DTLS.Insecure,
		Certificates:       []tls.Certificate{cert},
	}

	// Load the Root CA if specified
	if t.DTLS.RootCA != "" {
		caCert, err := os.ReadFile(t.DTLS.RootCA)
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

// UnmarshalYAML is a custom YAML unmarshaler for UdpTransport.
// It decodes the YAML configuration into the UdpTransport struct fields,
// mapping the common transport fields like Type, Enabled, IPv4, Port, and DTLS.
//
// Example YAML format:
//
//		type: udp
//		enabled: true
//		ipv4: "127.0.0.1"
//		port: 4242
//		dtls:
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
func (t *UdpTransport) UnmarshalYAML(value *yaml.Node) error {
	// Create a temporary struct to capture the common fields
	aux := struct {
		Type    types.TransportType `yaml:"type"`
		Enabled bool                `yaml:"enabled"`
		IPv4    string              `yaml:"ipv4"`
		Port    int                 `yaml:"port"`
		DTLS    *DTLS               `yaml:"dtls"`
	}{}

	// Unmarshal the common fields, including the nested DTLS config
	if err := value.Decode(&aux); err != nil {
		return fmt.Errorf("failed to unmarshal UDP transport fields: %w", err)
	}

	// Assign values to the actual struct
	t.Type = aux.Type
	t.Enabled = aux.Enabled
	t.IPv4 = aux.IPv4
	t.Port = aux.Port
	t.DTLS = aux.DTLS

	return nil
}
