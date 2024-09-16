package config

import (
	"fmt"
	"github.com/unpackdev/fdb/types"
	"gopkg.in/yaml.v3"
)

// TransportConfig defines an interface for transport configurations.
// Each transport implementation (e.g., DummyTransport, UdsTransport)
// must implement this interface to provide its specific transport type.
type TransportConfig interface {
	// GetTransportType returns the type of transport (e.g., UDS, Dummy).
	GetTransportType() types.TransportType
}

// Transport holds the generic transport configuration for different types
// of communication protocols, including UDS, QUIC, and Dummy transports.
//
// This struct is used to unmarshal transport configuration from YAML and
// determine which specific transport type to instantiate based on the type field.
type Transport struct {
	// Type defines the type of transport being used (e.g., UDS, Dummy, QUIC).
	Type types.TransportType `yaml:"type"`

	// Enabled indicates whether the transport is enabled or disabled.
	Enabled bool `yaml:"enabled"`

	// Config holds the specific configuration for the given transport type.
	// This is populated dynamically based on the Type field during unmarshalling.
	Config TransportConfig `yaml:"-"`
}

// TLS holds the TLS configuration used by the transport if needed. While
// Unix Domain Sockets typically don't use TLS, other transports like QUIC
// may require it for secure communication.
type TLS struct {
	// Insecure determines whether to skip verifying the server's TLS certificate.
	Insecure bool `yaml:"insecure"`

	// Cert is the path to the TLS certificate used for encryption.
	Cert string `json:"cert"`

	// Key is the path to the private key corresponding to the TLS certificate.
	Key string `json:"key"`

	// RootCA is the path to the Root Certificate Authority used for validating the server's TLS certificate.
	RootCA string `json:"rootCa"`
}

// UnmarshalYAML unmarshals a YAML node into the Transport struct. It first decodes
// the common transport fields (Type, Enabled) and then dynamically unmarshals the
// specific transport configuration (DummyTransport, UdsTransport, QuicTransport)
// based on the transport type.
//
// Example YAML configuration:
//
//	type: uds
//	enabled: true
//	config:
//	  socket: /tmp/my-uds.sock
//
// Parameters:
//
//	value (*yaml.Node): The YAML node to be decoded.
//
// Returns:
//
//	error: Returns an error if unmarshaling fails; otherwise, nil.
func (t *Transport) UnmarshalYAML(value *yaml.Node) error {
	// Create a temporary struct to capture the common fields
	aux := struct {
		Type    types.TransportType `yaml:"type"`
		Enabled bool                `yaml:"enabled"`
		Config  yaml.Node           `yaml:"config"` // Capture the nested "config" as a raw YAML node
	}{}

	// Unmarshal the common fields, including the raw "config" node
	if err := value.Decode(&aux); err != nil {
		return fmt.Errorf("failed to unmarshal common transport fields: %w", err)
	}

	t.Type = aux.Type
	t.Enabled = aux.Enabled

	// Depending on the transport type, decode the "config" field into the appropriate struct
	switch t.Type {
	case types.DummyTransportType:
		var config DummyTransport
		if err := aux.Config.Decode(&config); err != nil {
			return fmt.Errorf("failed to unmarshal dummy transport config: %w", err)
		}
		config.Type = types.DummyTransportType
		t.Config = &config

	case types.UDSTransportType:
		var config UdsTransport
		if err := aux.Config.Decode(&config); err != nil {
			return fmt.Errorf("failed to unmarshal UDS transport config: %w", err)
		}
		config.Type = types.UDSTransportType
		t.Config = &config

	case types.QUICTransportType:
		var config QuicTransport
		if err := aux.Config.Decode(&config); err != nil {
			return fmt.Errorf("failed to unmarshal QUIC transport config: %w", err)
		}
		config.Type = types.QUICTransportType
		t.Config = &config

	case types.TCPTransportType:
		var config TcpTransport
		if err := aux.Config.Decode(&config); err != nil {
			return fmt.Errorf("failed to unmarshal TCP transport config: %w", err)
		}
		config.Type = types.TCPTransportType
		t.Config = &config
	case types.UDPTransportType:
		var config UdpTransport
		if err := aux.Config.Decode(&config); err != nil {
			return fmt.Errorf("failed to unmarshal UDP transport config: %w", err)
		}
		config.Type = types.UDPTransportType
		t.Config = &config
	default:
		return fmt.Errorf("unsupported transport type: %s", t.Type)
	}

	return nil
}
