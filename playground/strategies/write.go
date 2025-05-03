package strategies

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/playground/suite"
)

// WriteStrategy implements a strategy for testing write operations
// with configurable parameters to take advantage of the optimized BatchWriter
type WriteStrategy struct {
	logger       logger.Logger
	nodes        suite.TestNodes
	targetNode   *suite.TestNode
	workersCount int
	keyPrefix    string
	dataSizeKB   int
	opsPerSec    int
	totalOps     int
	wg           sync.WaitGroup
	cancel       context.CancelFunc
	mu           sync.Mutex
	completedOps int
	startTime    time.Time
}

// NewWriteStrategy creates a new WriteStrategy with the given parameters
func NewWriteStrategy(
	logger logger.Logger,
	nodes suite.TestNodes,
	opts ...WriteStrategyOption,
) *WriteStrategy {
	s := &WriteStrategy{
		logger:       logger,
		nodes:        nodes,
		workersCount: 10, // Default number of concurrent workers
		keyPrefix:    "test-key-",
		dataSizeKB:   10,   // Default to 10KB per write
		opsPerSec:    100,  // Default operations per second
		totalOps:     1000, // Default total operations
	}

	// Set default target node if nodes is not empty
	if len(nodes) > 0 {
		s.targetNode = nodes[0] // Default to first node
	}

	// Apply any options provided
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// WriteStrategyOption defines option functions for configuring the WriteStrategy
type WriteStrategyOption func(*WriteStrategy)

// WithTargetNode specifies which node to send writes to
func WithTargetNode(index int) WriteStrategyOption {
	return func(s *WriteStrategy) {
		if index >= 0 && index < len(s.nodes) {
			s.targetNode = s.nodes[index]
		}
	}
}

// WithWorkersCount sets the number of concurrent workers
func WithWorkersCount(count int) WriteStrategyOption {
	return func(s *WriteStrategy) {
		if count > 0 {
			s.workersCount = count
		}
	}
}

// WithDataSize sets the size of data to write in KB
func WithDataSize(sizeKB int) WriteStrategyOption {
	return func(s *WriteStrategy) {
		if sizeKB > 0 {
			s.dataSizeKB = sizeKB
		}
	}
}

// WithOperationsPerSecond sets the target operations per second
func WithOperationsPerSecond(ops int) WriteStrategyOption {
	return func(s *WriteStrategy) {
		if ops > 0 {
			s.opsPerSec = ops
		}
	}
}

// WithTotalOperations sets the total number of operations to perform
func WithTotalOperations(ops int) WriteStrategyOption {
	return func(s *WriteStrategy) {
		if ops > 0 {
			s.totalOps = ops
		}
	}
}

// Start begins executing the write strategy
func (s *WriteStrategy) Start(ctx context.Context) error {
	s.logger.Info("Starting write strategy",
		"workers", s.workersCount,
		"data_size_kb", s.dataSizeKB,
		"ops_per_sec", s.opsPerSec,
		"total_ops", s.totalOps,
		"target_node", s.targetNode.PeerID().String())

	// Create a new context we can cancel when Stop is called
	ctx, s.cancel = context.WithCancel(ctx)
	s.startTime = time.Now()
	s.completedOps = 0

	// Calculate delay between operations to achieve target ops/sec across all workers
	delayPerOp := time.Second / time.Duration(s.opsPerSec)
	opsPerWorker := s.totalOps / s.workersCount
	if opsPerWorker == 0 {
		opsPerWorker = 1
	}

	// Start workers
	for i := 0; i < s.workersCount; i++ {
		s.wg.Add(1)
		workerID := i
		go s.runWorker(ctx, workerID, opsPerWorker, delayPerOp)
	}

	// Start a goroutine to report progress
	go s.reportProgress(ctx)

	return nil
}

// Stop gracefully terminates the strategy
func (s *WriteStrategy) Stop() error {
	s.logger.Info("Stopping write strategy")

	if s.cancel != nil {
		s.cancel()
		s.wg.Wait()
		s.cancel = nil
	}

	// Calculate and log final metrics
	duration := time.Since(s.startTime)
	ops := s.completedOps
	opsPerSec := float64(ops) / duration.Seconds()

	s.logger.Info("Write strategy completed",
		"total_ops", ops,
		"duration_seconds", duration.Seconds(),
		"ops_per_sec", opsPerSec,
	)

	return nil
}

// runWorker performs write operations for a single worker
func (s *WriteStrategy) runWorker(ctx context.Context, id, ops int, delay time.Duration) {
	defer s.wg.Done()

	s.logger.Debug("Worker started", "worker_id", id)

	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for i := 0; i < ops; i++ {
		select {
		case <-ctx.Done():
			s.logger.Debug("Worker stopping due to context cancellation", "worker_id", id)
			return
		case <-ticker.C:
			if err := s.performWriteOperation(ctx, id, i); err != nil {
				s.logger.Error("Write operation failed", "worker_id", id, "error", err.Error())
			} else {
				s.mu.Lock()
				s.completedOps++
				s.mu.Unlock()
			}
		}
	}

	s.logger.Debug("Worker finished", "worker_id", id)
}

// performWriteOperation executes a single write operation
func (s *WriteStrategy) performWriteOperation(ctx context.Context, workerID, opID int) error {
	// Generate key and test data
	key := fmt.Sprintf("%s%d-%d", s.keyPrefix, workerID, opID)

	// Generate test data of specified size
	data, err := suite.GenerateTestDataKB(s.dataSizeKB, false)
	if err != nil {
		return fmt.Errorf("failed to generate test data: %w", err)
	}

	// TODO: Implement actual write operation using the client
	// This is a placeholder for the actual implementation that would use the
	// optimized BatchWriter component with its 2048 batch size and 100ms flush interval

	// For now, we'll just log the operation
	s.logger.Debug("Write operation",
		"key", key,
		"data_size", len(data))

	// Simulate successful write
	return nil
}

// reportProgress periodically logs progress information
func (s *WriteStrategy) reportProgress(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			completed := s.completedOps
			s.mu.Unlock()

			duration := time.Since(s.startTime)
			opsPerSec := float64(completed) / duration.Seconds()
			progress := float64(completed) / float64(s.totalOps) * 100

			s.logger.Info("Progress update",
				"completed_ops", completed,
				"total_ops", s.totalOps,
				"progress_pct", progress,
				"ops_per_sec", opsPerSec,
				"elapsed", duration.String())
		}
	}
}

// Info returns metadata about the strategy
func (s *WriteStrategy) Info() Info {
	return Info{
		Name:        "write",
		Description: "Tests write performance using the optimized BatchWriter (2048 batch size, 100ms flush interval)",
		DefaultArgs: map[string]any{
			"target_node":  0,    // Default to first node
			"workers":      10,   // Default number of concurrent workers
			"data_size_kb": 10,   // Default to 10KB per write
			"ops_per_sec":  100,  // Default operations per second
			"total_ops":    1000, // Default total operations
		},
	}
}

// CreateFn returns a function that can create new instances of this strategy
func (s *WriteStrategy) CreateFn() StrategyFn {
	return func(logger logger.Logger, nodes suite.TestNodes, args map[string]any) (Strategy, error) {
		// Parse options from args
		options := []WriteStrategyOption{}

		// Target node
		if val, ok := args["target_node"]; ok {
			if idx, ok := val.(int); ok {
				options = append(options, WithTargetNode(idx))
			}
		}

		// Workers
		if val, ok := args["workers"]; ok {
			if count, ok := val.(int); ok {
				options = append(options, WithWorkersCount(count))
			}
		}

		// Data size
		if val, ok := args["data_size_kb"]; ok {
			if size, ok := val.(int); ok {
				options = append(options, WithDataSize(size))
			}
		}

		// Operations per second
		if val, ok := args["ops_per_sec"]; ok {
			if ops, ok := val.(int); ok {
				options = append(options, WithOperationsPerSecond(ops))
			}
		}

		// Total operations
		if val, ok := args["total_ops"]; ok {
			if ops, ok := val.(int); ok {
				options = append(options, WithTotalOperations(ops))
			}
		}

		// Create and return the write strategy
		return NewWriteStrategy(logger, nodes, options...), nil
	}
}
