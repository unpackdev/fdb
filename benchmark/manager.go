package benchmark

import (
	"context"
	"fmt"
	"github.com/unpackdev/fdb"
)

// SuiteManager manages multiple benchmarking suites for different transport types.
type SuiteManager struct {
	fdb    *fdb.FDB
	Suites map[SuiteType]TransportSuite
}

// NewSuiteManager creates a new SuiteManager capable of managing multiple transport-specific suites.
func NewSuiteManager(fdb *fdb.FDB) *SuiteManager {
	manager := &SuiteManager{
		fdb:    fdb,
		Suites: make(map[SuiteType]TransportSuite),
	}

	// Register available suites
	manager.RegisterSuite(QUICSuite, NewQuicSuite(fdb, 500))
	manager.RegisterSuite(DummySuiteType, NewDummySuite(fdb, 500))
	manager.RegisterSuite(UDSSuiteType, NewUdsSuite(fdb, 500))
	manager.RegisterSuite(TCPSuiteType, NewTcpSuite(fdb, 500))
	manager.RegisterSuite(UDPSuiteType, NewUdpSuite(fdb, 500))

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

// RunWriteBenchmark executes the write benchmarking logic for the specified SuiteType.
func (sm *SuiteManager) RunWriteBenchmark(ctx context.Context, suiteType SuiteType, numClients int, numMessagesPerClient int, report *Report) error {
	suite, exists := sm.Suites[suiteType]
	if !exists {
		return fmt.Errorf("suite type %s not found", suiteType)
	}
	return suite.RunWriteBenchmark(ctx, numClients, numMessagesPerClient, report)
}

// RunReadBenchmark executes the read benchmarking logic for the specified SuiteType.
func (sm *SuiteManager) RunReadBenchmark(ctx context.Context, suiteType SuiteType, numClients int, numMessagesPerClient int, report *Report) error {
	suite, exists := sm.Suites[suiteType]
	if !exists {
		return fmt.Errorf("suite type %s not found", suiteType)
	}
	return suite.RunReadBenchmark(ctx, numClients, numMessagesPerClient, report)
}
