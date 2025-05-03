package config

import (
	"gopkg.in/yaml.v3"
)

// Rpc holds the configuration for the RPC protocol.
type Rpc struct {
	Transport   TcpTransport `yaml:"transport" json:"transport" mapstructure:"transport"`
	PoolMaxSize int          `yaml:"poolMaxSize" json:"pool_size" mapstructure:"pool_size"`
}

// UnmarshalYAML custom unmarshaler for RpcConfig.
func (c *Rpc) UnmarshalYAML(value *yaml.Node) error {
	type rawRpcConfig Rpc
	var raw rawRpcConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*c = Rpc(raw)
	return nil
}
