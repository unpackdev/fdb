package logger

import (
	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/config"
	"syscall"
)

// Global logger instance and mutex for thread-safe operations
var (
	globalLogger Logger
	mu           deadlock.RWMutex
)

// SetGlobalLogger sets the global logger instance.
// It should be called once during application initialization.
func SetGlobalLogger(l Logger) error {
	mu.Lock()
	defer mu.Unlock()
	globalLogger = l
	return nil
}

// G retrieves the global logger instance.
// Returns a no-op logger if no global logger is set.
func G() Logger {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		return globalLogger
	}
	return NewNoOpLogger()
}

// InitializeGlobalLogger initializes the global logger based on the provided configuration.
// It should be called once during application startup.
func InitializeGlobalLogger(cfg config.Logger) (Logger, error) {
	l, err := Factory(cfg)
	if err != nil {
		return nil, err
	}
	if err := SetGlobalLogger(l); err != nil {
		return nil, err
	}
	return l, nil
}

// Sync flushes any buffered log entries.
// It should be called before application exit to ensure all logs are written.
func Sync() error {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		if zapLogger, ok := globalLogger.(*ZapLogger); ok && zapLogger.sugaredLogger != nil {
			err := zapLogger.sugaredLogger.Sync()
			if err != nil {
				// Ignore the "invalid argument" error from syncing stderr/stdout
				// The Sync() method attempts to flush any buffered log entries to the underlying file
				// descriptor, which might not be valid or supported for stderr and stdout in certain environments.
				// The error invalid argument is returned when Sync() is called on a file descriptor that does
				// not support syncing, such as /dev/stderr.
				if errors.Is(err, syscall.EINVAL) {
					return nil
				}
				return err
			}
		}
	}
	return nil
}
