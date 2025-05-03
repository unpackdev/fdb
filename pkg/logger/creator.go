package logger

import (
	"errors"

	"github.com/unpackdev/fdb/pkg/config"

	"strings"
)

// CreateLogger creates logger instances based on the configuration.
func CreateLogger(nodeId string, cfg config.Logger) (Logger, error) {
	if !cfg.Enabled {
		return NewNoOpLogger(), nil
	}

	// Currently only supports Zap as a provider.
	// Extend this function to support more providers.
	switch strings.ToLower(cfg.Environment) {
	case "production", "development":
		return NewZapLogger(nodeId, cfg)
	default:
		return nil, errors.New("unsupported environment for logger")
	}
}
