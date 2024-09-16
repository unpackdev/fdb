package benchmark

import (
	"context"
	"fmt"
	"github.com/unpackdev/fdb"
)

// SuiteManager manages multiple benchmarking suites for different transport types.
type SuiteManager struct {
	fdbInstance *fdb.FDB
	Suites      map[SuiteType]TransportSuite
}

// NewSuiteManager creates a new SuiteManager capable of managing multiple transport-specific suites.
func NewSuiteManager(fdbInstance *fdb.FDB) *SuiteManager {
	manager := &SuiteManager{
		fdbInstance: fdbInstance,
		Suites:      make(map[SuiteType]TransportSuite),
	}

	// Register available suites
	manager.RegisterSuite(QUICSuite, NewQuicSuite(fdbInstance))
	manager.RegisterSuite(DummySuiteType, NewDummySuite(fdbInstance))

	// Future: Add other suites like UDS here

	return manager
}

// RegisterSuite registers a transport-specific suite with the manager.
func (sm *SuiteManager) RegisterSuite(suiteType SuiteType, suite TransportSuite) {
	sm.Suites[suiteType] = suite
}

// Start starts the suite for the specified SuiteType.
func (sm *SuiteManager) Start(ctx context.Context, suiteType SuiteType) error {
	suite, exists := sm.Suites[suiteType]
	if !exists {
		return fmt.Errorf("suite type %s not found", suiteType)
	}
	return suite.Start(ctx)
}

// Stop stops the suite for the specified SuiteType.
func (sm *SuiteManager) Stop(ctx context.Context, suiteType SuiteType) error {
	if suite, exists := sm.Suites[suiteType]; exists {
		if err := suite.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Run executes the benchmarking logic for the specified SuiteType.
func (sm *SuiteManager) Run(ctx context.Context, suiteType SuiteType, numClients int, numMessagesPerClient int) error {
	suite, exists := sm.Suites[suiteType]
	if !exists {
		return fmt.Errorf("suite type %s not found", suiteType)
	}
	return suite.Run(ctx)
}
