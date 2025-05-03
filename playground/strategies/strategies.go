package strategies

import (
	"context"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/playground/suite"
)

// Info contains metadata about a strategy
type Info struct {
	// Name is the unique identifier for the strategy
	Name string

	// Description explains what the strategy does
	Description string

	// DefaultArgs contains the default parameter values
	DefaultArgs map[string]any
}

// StrategyFn is a function that creates a strategy instance
type StrategyFn func(logger logger.Logger, nodes suite.TestNodes, args map[string]any) (Strategy, error)

// Strategy defines the interface for playground strategies
// that can be started and stopped to test different behaviors
// of the FDB network.
type Strategy interface {
	// Start begins executing the strategy with the provided context
	Start(ctx context.Context) error

	// Stop gracefully terminates the strategy
	Stop() error

	// Info returns metadata about the strategy
	Info() Info

	// CreateFn returns a function that can create new instances of this strategy
	CreateFn() StrategyFn
}
