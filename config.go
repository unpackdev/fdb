package fdb

type MdbxNode struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type MdbxNodes []MdbxNode

type Config struct {
	MdbxNodes MdbxNodes `yaml:"nodes"`
}

func (c *Config) GetMdbxNodeByName(name string) *MdbxNode {
	for _, node := range c.MdbxNodes {
		if node.Name == name {
			return &node
		}
	}
	return nil
}
