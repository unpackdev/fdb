package config

import "github.com/unpackdev/fdb/types"

type Config struct {
	Transports []Transport `yaml:"transports"`
	MdbxNodes  MdbxNodes   `yaml:"nodes"`
}

func (c Config) Validate() error {
	return nil
}

func (c Config) GetTransportByType(transportType types.TransportType) *Transport {
	for _, t := range c.Transports {
		if t.Type == transportType {
			return &t
		}
	}
	return nil
}
