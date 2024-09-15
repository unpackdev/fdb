package benchmark

import (
	"context"
	"fmt"
	"github.com/unpackdev/fdb"
)

// SuiteManager manages multiple benchmarking suites for different transport types.
type SuiteManager struct {
	fdbInstance *fdb.FDB
	suites      map[SuiteType]TransportSuite
}

// NewSuiteManager creates a new suite manager capable of managing multiple transport-specific suites.
func NewSuiteManager(fdbInstance *fdb.FDB) *SuiteManager {
	manager := &SuiteManager{
		fdbInstance: fdbInstance,
		suites:      make(map[SuiteType]TransportSuite),
	}

	// Register available suites
	manager.RegisterSuite(QUICSuite, NewQuicSuite(fdbInstance))
	// Future: Register more suites like UDS here

	return manager
}

// RegisterSuite registers a transport-specific suite with the manager.
func (sm *SuiteManager) RegisterSuite(suiteType SuiteType, suite TransportSuite) {
	sm.suites[suiteType] = suite
}

// Start starts the suite for the specified SuiteType.
func (sm *SuiteManager) Start(suiteType SuiteType) error {
	suite, exists := sm.suites[suiteType]
	if !exists {
		return fmt.Errorf("%w: %s", ErrInvalidSuiteType, suiteType)
	}
	return suite.Start()
}

// Stop stops the suite for the specified SuiteType.
func (sm *SuiteManager) Stop(suiteType SuiteType) {
	suite, exists := sm.suites[suiteType]
	if exists {
		suite.Stop()
	}
}

// Run runs the client benchmark for the specified SuiteType.
func (sm *SuiteManager) Run(ctx context.Context, suiteType SuiteType) error {
	suite, exists := sm.suites[suiteType]
	if !exists {
		return fmt.Errorf("%w: %s", ErrInvalidSuiteType, suiteType)
	}
	return suite.Run(ctx)
}
