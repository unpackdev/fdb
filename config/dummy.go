package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/unpackdev/fdb/types"
	"os"
)

// DummyTransport implements TransportConfig
type DummyTransport struct {
	Type    types.TransportType `yaml:"type" json:"type" mapstructure:"type"`
	Enabled bool                `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	IPv4    string              `yaml:"ipv4" json:"ipv4" mapstructure:"ipv4"`
	Port    int                 `yaml:"port" json:"port" mapstructure:"port"`
	TLS     TLS                 `yaml:"tls" json:"tls" mapstructure:"tls"`
}

func (q DummyTransport) Addr() string {
	return fmt.Sprintf("%s:%d", q.IPv4, q.Port)
}

func (q DummyTransport) GetTransportType() types.TransportType {
	return q.Type
}

func (q DummyTransport) GetTLSConfig() (*tls.Config, error) {
	// Load the certificate and key
	cert, err := tls.LoadX509KeyPair(q.TLS.Cert, q.TLS.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate and key: %w", err)
	}

	// Create the TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// If a RootCA file is specified, load it
	if q.TLS.RootCA != "" {
		caCert, err := os.ReadFile(q.TLS.RootCA)
		if err != nil {
			return nil, fmt.Errorf("failed to read root CA file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append root CA certificates")
		}

		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}
