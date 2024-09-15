package config

// MdbxNode and MdbxNodes as before
type MdbxNode struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type MdbxNodes []MdbxNode

func (c Config) GetMdbxNodeByName(name string) *MdbxNode {
	for _, node := range c.MdbxNodes {
		if node.Name == name {
			return &node
		}
	}
	return nil
}
