package config

// MdbxNode represents the configuration for an individual MDBX node. Each node
// corresponds to an instance of the MDBX database, with specific configurations for
// file path, size, and performance optimizations.
type MdbxNode struct {
	// Name is the identifier for the MDBX node, allowing the system to distinguish between multiple nodes.
	Name string `yaml:"name"`

	// Path specifies the file system path where the MDBX database files are stored.
	Path string `yaml:"path"`

	// MaxReaders defines the maximum number of readers allowed for the MDBX instance.
	// This controls how many concurrent read transactions can be active at the same time.
	MaxReaders int `yaml:"maxReaders"`

	// MaxSize defines the maximum size of the MDBX database in bytes. This is the upper limit
	// on the size the database can grow to on disk.
	MaxSize int64 `yaml:"maxSize"`

	// MinSize defines the minimum size of the MDBX database in bytes. The database will allocate
	// at least this amount of space on disk.
	MinSize int64 `yaml:"minSize"`

	// GrowthStep specifies the size in bytes by which the MDBX database will grow when it needs more space.
	// This controls how efficiently the database expands on disk.
	GrowthStep int64 `yaml:"growthStep"`

	// FilePermissions sets the file system permissions for the MDBX database files. It defaults to 0600,
	// which grants read and write access to the file owner only.
	FilePermissions uint `yaml:"filePermissions"`
}

// Mdbx represents the global MDBX configuration. It enables or disables MDBX functionality
// and holds a list of MDBX nodes, each of which corresponds to a specific MDBX instance configuration.
type Mdbx struct {
	// Enabled determines if MDBX is enabled for the application.
	Enabled bool `yaml:"enabled"`

	// Nodes is a list of MDBX nodes. Each node contains its own configuration, allowing
	// multiple MDBX databases to be configured with different paths, sizes, and performance settings.
	Nodes []MdbxNode `yaml:"nodes"`
}

// GetMdbxNodeByName searches for an MDBX node by its name and returns the corresponding MdbxNode configuration.
// This method is useful for discovering specific node configurations based on the node's name.
//
// Example usage:
//
//	nodeConfig := config.GetMdbxNodeByName("node1")
//	if nodeConfig == nil {
//	    log.Fatalf("MDBX node not found")
//	}
//
// Parameters:
//
//	name (string): The name of the MDBX node to search for.
//
// Returns:
//
//	*MdbxNode: Returns a pointer to the MdbxNode if found, or nil if no node matches the provided name.
func (c Config) GetMdbxNodeByName(name string) *MdbxNode {
	for _, node := range c.Mdbx.Nodes {
		if node.Name == name {
			return &node
		}
	}
	return nil
}
