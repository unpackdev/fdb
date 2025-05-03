package strategies

import (
	"context"
	"sync"
	"time"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/playground/suite"
)

// NetworkStrategy tests the network connectivity between nodes
type NetworkStrategy struct {
	logger      logger.Logger
	nodes       suite.TestNodes
	doneCh      chan struct{}
	cancel      context.CancelFunc
	duration    time.Duration
	wg          sync.WaitGroup
	pingTimeout time.Duration
}

// NewNetworkStrategy creates a new network strategy
func NewNetworkStrategy(logger logger.Logger, nodes suite.TestNodes) *NetworkStrategy {
	return &NetworkStrategy{
		logger:      logger,
		nodes:       nodes,
		doneCh:      make(chan struct{}),
		duration:    60 * time.Second, // Run for 1 minute by default
		pingTimeout: 5 * time.Second,
	}
}

// Info returns information about the network strategy
func (s *NetworkStrategy) Info() Info {
	return Info{
		Name:        "network",
		Description: "Tests network connectivity between nodes",
		DefaultArgs: map[string]any{
			"duration":     60, // seconds
			"ping_timeout": 5,  // seconds
		},
		ArgMappings: []ArgMapping{
			{
				Flag:         "duration",
				ParamKey:     "duration",
				Description:  "Duration to run the network test in seconds",
				DefaultValue: 60,
			},
			{
				Flag:         "ping-timeout",
				ParamKey:     "ping_timeout",
				Description:  "Timeout for each ping operation in seconds",
				DefaultValue: 5,
			},
		},
	}
}

// Start begins the network connectivity testing
func (s *NetworkStrategy) Start(ctx context.Context) error {
	s.logger.Info("Starting network connectivity testing",
		"node_count", len(s.nodes),
		"duration", s.duration.String(),
		"ping_timeout", s.pingTimeout.String())

	// Create a new context we can cancel when Stop is called
	ctx, s.cancel = context.WithCancel(ctx)
	
	s.wg.Add(1)
	go s.runConnectivityTest(ctx)

	// Set up the completion timer
	go func() {
		timer := time.NewTimer(s.duration)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.logger.Info("Network test completed successfully")
			close(s.doneCh)
			if s.cancel != nil {
				s.cancel()
			}
		}
	}()

	return nil
}

// Stop halts the network strategy
func (s *NetworkStrategy) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

// CompletionCh returns a channel that is closed when the strategy completes
func (s *NetworkStrategy) CompletionCh() <-chan struct{} {
	return s.doneCh
}

// CreateFn returns a function that can create new instances of this strategy
func (s *NetworkStrategy) CreateFn() StrategyFn {
	return func(logger logger.Logger, nodes suite.TestNodes, args map[string]any) (Strategy, error) {
		strategy := NewNetworkStrategy(logger, nodes)
		
		// Apply configuration from args
		if durationSec, ok := args["duration"].(int); ok && durationSec > 0 {
			strategy.duration = time.Duration(durationSec) * time.Second
		}
		
		if timeout, ok := args["ping_timeout"].(int); ok && timeout > 0 {
			strategy.pingTimeout = time.Duration(timeout) * time.Second
		}
		
		return strategy, nil
	}
}

// runConnectivityTest periodically checks connectivity between nodes
func (s *NetworkStrategy) runConnectivityTest(ctx context.Context) {
	defer s.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkConnectivity()
		}
	}
}

// checkConnectivity verifies that all nodes can see each other
func (s *NetworkStrategy) checkConnectivity() {
	totalNodes := len(s.nodes)
	s.logger.Info("Checking network connectivity", "total_nodes", totalNodes)
	
	for i, node := range s.nodes {
		peers := node.Node().Network().Host().Network().Peers()
		s.logger.Info("Node connectivity status", 
			"node_index", i,
			"peer_id", node.PeerID().String(),
			"connected_peers", len(peers),
			"expected_peers", totalNodes-1) // Exclude self
	}
}
