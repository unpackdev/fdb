package playground

import (
	"context"
	"fmt"
	"sync"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/playground/strategies"
	"github.com/unpackdev/fdb/playground/suite"
)

// strategyEntry represents a registered strategy in the registry
type strategyEntry struct {
	// Strategy is the actual strategy instance (prototype)
	Strategy strategies.Strategy
}

// Registry manages the collection of available test strategies
type Registry struct {
	mu         sync.RWMutex
	strategies map[string]strategyEntry
	logger     logger.Logger
}

// NewRegistry creates a new strategy registry
func NewRegistry(logger logger.Logger) *Registry {
	return &Registry{
		logger:     logger,
		strategies: make(map[string]strategyEntry),
	}
}

// Register adds a new strategy to the registry
func (r *Registry) Register(strategy strategies.Strategy) error {
	if strategy == nil {
		return fmt.Errorf("strategy cannot be nil")
	}

	info := strategy.Info()
	if info.Name == "" {
		return fmt.Errorf("strategy name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicates
	if _, exists := r.strategies[info.Name]; exists {
		return fmt.Errorf("strategy with name '%s' is already registered", info.Name)
	}

	r.strategies[info.Name] = strategyEntry{
		Strategy: strategy,
	}
	return nil
}

// List returns information about all registered strategies
func (r *Registry) List() []strategies.Info {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]strategies.Info, 0, len(r.strategies))
	for _, entry := range r.strategies {
		result = append(result, entry.Strategy.Info())
	}

	return result
}

// GetStrategy returns the strategy info for the given name
func (r *Registry) GetStrategy(name string) (strategies.Info, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, found := r.strategies[name]
	if !found {
		return strategies.Info{}, false
	}
	return entry.Strategy.Info(), found
}

// CreateStrategy instantiates a new strategy by name
func (r *Registry) CreateStrategy(name string, logger logger.Logger, nodes suite.TestNodes, args map[string]any) (strategies.Strategy, error) {
	r.mu.RLock()
	entry, found := r.strategies[name]
	r.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("strategy '%s' not found", name)
	}

	// Get the strategy info for default args
	info := entry.Strategy.Info()

	// Merge default args with provided args
	mergedArgs := make(map[string]any)
	for k, v := range info.DefaultArgs {
		mergedArgs[k] = v
	}
	for k, v := range args {
		mergedArgs[k] = v
	}

	// Use the strategy's CreateFn to instantiate a new instance
	return entry.Strategy.CreateFn()(logger, nodes, mergedArgs)
}

// RegisterWriteStrategy registers the built-in write strategy
func (r *Registry) RegisterWriteStrategy() {
	// Create a prototype instance of the WriteStrategy
	writeStrategy := strategies.NewWriteStrategy(r.logger, nil) // Nodes will be provided later

	// Register the strategy
	err := r.Register(writeStrategy)
	if err != nil {
		r.logger.Error("Failed to register write strategy", "error", err.Error())
	}
}

// RunStrategy creates, starts, and manages a strategy by name
func (r *Registry) RunStrategy(ctx context.Context, name string, logger logger.Logger, nodes suite.TestNodes, args map[string]any) error {
	strategy, err := r.CreateStrategy(name, logger, nodes, args)
	if err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start the strategy
	if err := strategy.Start(ctx); err != nil {
		return fmt.Errorf("failed to start strategy: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Stop the strategy
	if err := strategy.Stop(); err != nil {
		return fmt.Errorf("failed to stop strategy: %w", err)
	}

	return nil
}
