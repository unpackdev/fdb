package config

import (
	"fmt"
	"github.com/unpackdev/fdb/types"
	"gopkg.in/yaml.v3"
)

// UdsTransport represents the configuration for a Unix Domain Socket (UDS) transport.
// It implements the TransportConfig interface to be used in applications requiring UDS
// transport configuration for inter-process communication (IPC) on the same machine.
type UdsTransport struct {
	// Type defines the transport type, typically represented as types.TransportTypeUDS for UDS.
	Type types.TransportType `yaml:"type" json:"type" mapstructure:"type"`

	// Enabled determines if this transport configuration is active or not.
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`

	// Socket is the file path to the Unix Domain Socket. This field is required to establish
	// UDS communication, representing the location where the socket is created.
	Socket string `yaml:"socket" json:"socket" mapstructure:"socket"`
}

// Addr returns the address (file path) of the UDS socket.
// This method implements the Addr() method from the TransportConfig interface.
//
// Example usage:
//
//	socketPath := udsTransport.Addr()
//
// Returns:
//
//	string: The Unix Domain Socket file path.
func (u UdsTransport) Addr() string {
	return u.Socket
}

// GetTransportType returns the type of transport, which is typically types.TransportTypeUDS
// for Unix Domain Socket communication.
//
// Example usage:
//
//	transportType := udsTransport.GetTransportType()
//
// Returns:
//
//	types.TransportType: The type of transport.
func (u UdsTransport) GetTransportType() types.TransportType {
	return u.Type
}

// UnmarshalYAML provides custom unmarshaling logic for UdsTransport from YAML format.
// It reads the YAML fields and assigns them to the UdsTransport struct, ensuring proper
// decoding of all transport fields. This method is useful when loading configurations from
// a YAML file.
//
// Example YAML format:
//
//	type: uds
//	enabled: true
//	socket: /tmp/my-uds.sock
//
// Parameters:
//
//	value (*yaml.Node): The YAML node to be decoded.
//
// Returns:
//
//	error: Returns an error if unmarshaling fails; otherwise, nil.
func (u *UdsTransport) UnmarshalYAML(value *yaml.Node) error {
	aux := struct {
		Type    types.TransportType `yaml:"type"`
		Enabled bool                `yaml:"enabled"`
		Socket  string              `yaml:"socket"`
	}{}

	if err := value.Decode(&aux); err != nil {
		return fmt.Errorf("failed to unmarshal UDS transport fields: %w", err)
	}

	u.Type = aux.Type
	u.Enabled = aux.Enabled
	u.Socket = aux.Socket
	return nil
}
