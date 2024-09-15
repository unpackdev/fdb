package fdb

type MdbxNode struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type MdbxNodes []MdbxNode

type Transport struct {
	Type    TransportType `yaml:"type" json:"type" mapstructure:"type"`
	Enabled bool          `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	IPv4    string        `yaml:"ipv4" json:"ipv4" mapstructure:"ipv4"`
	Port    int           `yaml:"port" json:"port" mapstructure:"port"`
}

type Config struct {
	Transports []Transport `yaml:"transports"`
	MdbxNodes  MdbxNodes   `yaml:"nodes"`
}

func (c Config) Validate() error {
	return nil
}

func (c Config) GetTransportByType(transportType TransportType) *Transport {
	for _, t := range c.Transports {
		if t.Type == transportType {
			return &t
		}
	}
	return nil
}

func (c Config) GetMdbxNodeByName(name string) *MdbxNode {
	for _, node := range c.MdbxNodes {
		if node.Name == name {
			return &node
		}
	}
	return nil
}
