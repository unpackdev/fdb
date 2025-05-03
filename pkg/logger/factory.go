package logger

import (
	"errors"

	"github.com/unpackdev/fdb/pkg/config"

	"strings"
)

// Factory creates logger instances based on the configuration.
func Factory(cfg config.Logger) (Logger, error) {
	if !cfg.Enabled {
		return NewNoOpLogger(), nil
	}

	// Currently only supports Zap as a provider.
	// Extend this function to support more providers.
	switch strings.ToLower(cfg.Environment) {
	case "production", "development":
		return NewZapLogger(cfg)
	default:
		return nil, errors.New("unsupported environment for logger")
	}
}
