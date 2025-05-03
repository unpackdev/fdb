package config

import "fmt"

// Pprof represents the configuration for enabling pprof profiling in Go services.
// It allows you to configure the pprof server, including whether it is enabled,
// the service name, and the address on which the pprof server should run.
type Pprof struct {
	// Enabled determines if pprof profiling is enabled for the service.
	Enabled bool `yaml:"enabled"`

	// Name is the name of the service that uses pprof for profiling.
	// This allows the configuration to be tied to a specific service.
	Name string `yaml:"name"`

	// Addr is the address where the pprof server will be hosted.
	// Typically, this will be in the form of an IP address and port (e.g., "127.0.0.1:6060").
	Addr string `yaml:"addr"`
}

// GetPprofByServiceTag searches through the list of pprof configurations to find
// the configuration corresponding to the specified service tag (name).
// This method helps discover pprof settings for a specific service based on its name.
//
// Example usage:
//
//	pprofConfig, err := config.GetPprofByServiceTag("my-service")
//	if err != nil {
//	    log.Fatalf("Failed to get pprof config: %v", err)
//	}
//
// Parameters:
//
//	service (string): The service name (tag) to search for in the pprof configurations.
//
// Returns:
//
//	*Pprof: Returns a pointer to the Pprof configuration if found.
//	error: Returns an error if no matching service tag is found.
func (c Config) GetPprofByServiceTag(service string) (*Pprof, error) {
	for _, opts := range c.Pprof {
		if opts.Name == service {
			return &opts, nil
		}
	}

	return nil, fmt.Errorf("could not discover pprof service by tag: %s", service)
}
