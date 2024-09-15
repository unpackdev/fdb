package config

import "github.com/unpackdev/fdb/pkg/types"

type TransportConfig interface {
	GetTransportType() types.TransportType
}

type Transport struct {
	Type    types.TransportType
	Enabled bool
	Config  TransportConfig
}

// QuicTransport implements TransportConfig
type QuicTransport struct {
	Type    types.TransportType `yaml:"type" json:"type" mapstructure:"type"`
	Enabled bool                `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	IPv4    string              `yaml:"ipv4" json:"ipv4" mapstructure:"ipv4"`
	Port    int                 `yaml:"port" json:"port" mapstructure:"port"`
}

func (q QuicTransport) GetTransportType() types.TransportType {
	return q.Type
}

// UdsTransport implements TransportConfig
type UdsTransport struct {
	Type    types.TransportType `yaml:"type" json:"type" mapstructure:"type"`
	Enabled bool                `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	IPv4    string              `yaml:"ipv4" json:"ipv4" mapstructure:"ipv4"`
	Port    int                 `yaml:"port" json:"port" mapstructure:"port"`
}

func (u UdsTransport) GetTransportType() types.TransportType {
	return u.Type
}
