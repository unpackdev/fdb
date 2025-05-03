package strategies

import (
	"context"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/playground/suite"
	"github.com/urfave/cli/v2"
)

// ArgMapping defines how CLI flags map to strategy parameters
type ArgMapping struct {
	// Flag is the name of the flag in the CLI (e.g. "data-size")
	Flag string

	// ParamKey is the key in the args map passed to the strategy (e.g. "data_size_kb")
	ParamKey string

	// Description is a user-friendly description of the parameter
	Description string

	// DefaultValue is the default value for this parameter
	DefaultValue any

	// Required indicates if this parameter must be provided
	Required bool
}

// Info contains metadata about a strategy
type Info struct {
	// Name is the unique identifier for the strategy
	Name string

	// Description explains what the strategy does
	Description string

	// DefaultArgs contains the default parameter values
	DefaultArgs map[string]any

	// ArgMappings defines how CLI flags map to strategy parameters
	ArgMappings []ArgMapping
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

	// CompletionCh returns a channel that will be closed when the strategy's work is done
	// This allows external callers to detect when a strategy has naturally completed
	CompletionCh() <-chan struct{}
}

// ParseArgs converts CLI context args to strategy args based on the mappings
func ParseArgs(cliCtx *cli.Context, info Info) map[string]any {
	args := make(map[string]any)

	// Add default values from DefaultArgs
	for k, v := range info.DefaultArgs {
		args[k] = v
	}

	// Parse args from CLI flags based on the mappings
	for _, mapping := range info.ArgMappings {
		if cliCtx.IsSet(mapping.Flag) {
			// Get the value based on type inference
			switch mapping.DefaultValue.(type) {
			case int:
				args[mapping.ParamKey] = cliCtx.Int(mapping.Flag)
			case string:
				args[mapping.ParamKey] = cliCtx.String(mapping.Flag)
			case bool:
				args[mapping.ParamKey] = cliCtx.Bool(mapping.Flag)
			case float64:
				args[mapping.ParamKey] = cliCtx.Float64(mapping.Flag)
			default:
				// For other types, just use the string value
				args[mapping.ParamKey] = cliCtx.String(mapping.Flag)
			}
		}
	}

	return args
}
