package config

import (
	"github.com/unpackdev/fdb/types"
)

type TransportConfig interface {
	GetTransportType() types.TransportType
}

type Transport struct {
	Type    types.TransportType
	Enabled bool
	Config  TransportConfig
}

type TLS struct {
	Insecure bool   `yaml:"insecure"`
	Cert     string `json:"cert"`
	Key      string `json:"key"`
	RootCA   string `json:"rootCa"`
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
