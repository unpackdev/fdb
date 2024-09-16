package benchmark

import (
	"context"
	"errors"
)

// SuiteType represents different types of benchmarking suites (e.g., QUIC, UDS, etc.)
type SuiteType string

const (
	QUICSuite      SuiteType = "quic"
	UDSSuiteType   SuiteType = "uds" // Example for future transport suites
	TCPSuiteType   SuiteType = "tcp"
	DummySuiteType SuiteType = "dummy"
)

// ErrInvalidSuiteType is returned when an unsupported SuiteType is provided.
var ErrInvalidSuiteType = errors.New("invalid suite type")

// TransportSuite defines a common interface that all transport-specific suites must implement.
type TransportSuite interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RunWriteBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error
	RunReadBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error
}
