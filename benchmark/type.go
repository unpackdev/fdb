package benchmark

import (
	"context"
	"errors"
)

// SuiteType represents different types of benchmarking suites (e.g., QUIC, UDS, etc.)
type SuiteType string

const (
	QUICSuite SuiteType = "quic"
	UDSSuite  SuiteType = "uds" // Example for future transport suites
)

// ErrInvalidSuiteType is returned when an unsupported SuiteType is provided.
var ErrInvalidSuiteType = errors.New("invalid suite type")

// TransportSuite defines a common interface that all transport-specific suites must implement.
type TransportSuite interface {
	Start() error
	Stop()
	SetupClient(ctx context.Context) error
	Run(ctx context.Context) error
}
