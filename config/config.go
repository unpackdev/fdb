package config

import (
	"fmt"
	"github.com/unpackdev/fdb/types"
	"gopkg.in/yaml.v3"
	"os"
)

// Config represents the overall application configuration, which includes logging,
// transports, MDBX nodes, and pprof profiling options. This struct aggregates
// all major configuration sections for easy management and access throughout the application.
type Config struct {
	// Logger holds the configuration for the logging system, including log level and environment.
	Logger Logger `yaml:"logger"`

	// Transports is a list of various transport configurations (e.g., Dummy, UDS, QUIC).
	// Each transport has its own specific configuration settings.
	Transports []Transport `yaml:"transports"`

	// Mdbx contains the configuration for MDBX database nodes, including paths, sizes, and permissions.
	Mdbx Mdbx `yaml:"mdbx"`

	// Pprof is a list of pprof profiling configurations, each tied to a specific service or subsystem.
	Pprof []Pprof `yaml:"pprof"`
}

// Validate checks the integrity of the loaded configuration.
// Currently, it returns nil, but you can extend it to perform validation on
// the various configuration fields to ensure they are set correctly.
//
// Example usage:
//
//	if err := config.Validate(); err != nil {
//	    log.Fatalf("Invalid configuration: %v", err)
//	}
//
// Returns:
//
//	error: Returns nil if the configuration is valid, or an error if validation fails.
func (c Config) Validate() error {
	return nil
}

// GetTransportByType retrieves a specific transport configuration based on its type (e.g., UDS, Dummy).
// This method allows you to access a particular transport configuration when multiple transports are defined.
//
// Example usage:
//
//	transport := config.GetTransportByType(types.DummyTransportType)
//	if transport == nil {
//	    log.Fatalf("Transport not found")
//	}
//
// Parameters:
//
//	transportType (types.TransportType): The type of transport to search for.
//
// Returns:
//
//	*Transport: Returns a pointer to the matching transport configuration if found, or nil if no match is found.
func (c Config) GetTransportByType(transportType types.TransportType) *Transport {
	for _, t := range c.Transports {
		if t.Type == transportType {
			return &t
		}
	}
	return nil
}

// LoadConfig loads the configuration from a YAML file into the Config struct.
// This function reads the specified YAML configuration file and unmarshals it into
// a Config struct, enabling structured access to configuration settings.
//
// Example usage:
//
//	config, err := LoadConfig("/path/to/config.yaml")
//	if err != nil {
//	    log.Fatalf("Failed to load config: %v", err)
//	}
//
// Parameters:
//
//	filename (string): The path to the YAML configuration file.
//
// Returns:
//
//	*Config: Returns a pointer to the Config struct containing the parsed configuration.
//	error: Returns an error if reading or unmarshaling the YAML file fails.
func LoadConfig(filename string) (*Config, error) {
	// Read the configuration file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal the YAML data into the Config struct
	var rawConfig Config
	err = yaml.Unmarshal(data, &rawConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	// Optional: Validate the loaded configuration
	if err = rawConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &rawConfig, nil
}
