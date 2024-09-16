// Package config provides the application configuration structures and utilities
// for loading, validating, and managing configuration settings. The package supports
// YAML-based configuration for various components such as logging, transports (e.g., Dummy,
// UDS, QUIC), MDBX nodes, and pprof profiling.
//
// This package includes:
//
//  1. **Logger Configuration**: Defines the settings for application logging, including
//     log levels and environment-based logging adjustments.
//
//  2. **Transport Configuration**: Manages different transport protocols and their settings.
//     This includes DummyTransport, UdsTransport, and QuicTransport, each with its own
//     specific configuration fields like IP address, port, and TLS settings.
//
//  3. **MDBX Configuration**: Provides configuration for MDBX database nodes, including
//     paths, file size limits, and file permissions.
//
//  4. **pprof Configuration**: Configures pprof profiling for performance analysis and
//     debugging, allowing specific services to enable pprof with an address to bind to.
//
// Example usage:
//
//	// Load the configuration from a file
//	config, err := config.LoadConfig("/path/to/config.yaml")
//	if err != nil {
//	    log.Fatalf("Failed to load configuration: %v", err)
//	}
//
//	// Access the logger configuration
//	if config.Logger.Enabled {
//	    setupLogger(config.Logger)
//	}
//
//	// Get a specific transport configuration by type
//	transport := config.GetTransportByType(types.UDSTransportType)
//	if transport == nil {
//	    log.Fatalf("UDS transport not found")
//	}
//
//	// Access MDBX configuration for a specific node
//	mdbxNode := config.GetMdbxNodeByName("node1")
//	if mdbxNode == nil {
//	    log.Fatalf("MDBX node not found")
//	}
//
//	// Access pprof configuration for a specific service
//	pprofConfig, err := config.GetPprofByServiceTag("serviceA")
//	if err != nil {
//	    log.Fatalf("Failed to get pprof config: %v", err)
//	}
//
// The package also provides methods to validate configuration after loading and ensures
// that settings are properly configured before the application starts.
//
// Configuration Structure:
// The configuration is stored in a YAML format and is loaded into the Config struct
// to allow easy access to different settings.
//
// Example YAML Configuration:
//
//	logger:
//	  enabled: true
//	  environment: "production"
//	  level: "info"
//
//	transports:
//	  - type: "dummy"
//	    enabled: true
//	    ipv4: "127.0.0.1"
//	    port: 8080
//	    tls:
//	      insecure: true
//
//	mdbx:
//	  enabled: true
//	  nodes:
//	    - name: "node1"
//	      path: "/data/mdbx"
//	      maxReaders: 100
//	      maxSize: 1073741824
//	      minSize: 10485760
//	      growthStep: 1048576
//	      filePermissions: 0600
//
//	pprof:
//	  - enabled: true
//	    name: "serviceA"
//	    addr: "localhost:6060"
//
// The config package centralizes application settings, providing a clean, consistent
// way to manage configurations across different components.
package config
