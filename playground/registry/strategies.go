package registry

import (
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/playground/strategies"
)

// RegistryFn is a function that creates a strategy prototype
type RegistryFn func(logger logger.Logger) strategies.Strategy

// AvailableStrategies is a map of all available strategies and their factory functions
// Add new strategies to this map to make them available in the playground
var AvailableStrategies = map[string]RegistryFn{
	"write": func(logger logger.Logger) strategies.Strategy {
		return strategies.NewWriteStrategy(logger, nil) // Nodes will be provided later
	},
	"network": func(logger logger.Logger) strategies.Strategy {
		return strategies.NewNetworkStrategy(logger, nil) // Nodes will be provided later
	},
}
