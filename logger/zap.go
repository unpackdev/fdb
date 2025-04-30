package logger

import (
	"errors"
	"strings"

	"github.com/unpackdev/fdb/config"

	"go.uber.org/zap"
)

// ZapLogger is an implementation of Logger using Uber's Zap.
type ZapLogger struct {
	sugaredLogger *zap.SugaredLogger
}

// NewZapLogger creates a new ZapLogger based on the provided configuration.
func NewZapLogger(cfg config.Logger) (*ZapLogger, error) {
	var zapCfg zap.Config
	switch strings.ToLower(cfg.Environment) {
	case "production":
		zapCfg = zap.NewProductionConfig()
	case "development":
		zapCfg = zap.NewDevelopmentConfig()
	default:
		return nil, errors.New("invalid environment; must be 'production' or 'development'")
	}

	// Set log level
	level := strings.ToLower(cfg.Level)
	switch level {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		return nil, errors.New("invalid log level; must be 'debug', 'info', 'warn', or 'error'")
	}

	logger, err := zapCfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, err
	}

	sugar := logger.Sugar()
	return &ZapLogger{sugaredLogger: sugar}, nil
}

// Debug logs a message at DebugLevel.
func (z *ZapLogger) Debug(msg string, keysAndValues ...any) {
	z.sugaredLogger.Debugw(msg, keysAndValues...)
}

// Info logs a message at InfoLevel.
func (z *ZapLogger) Info(msg string, keysAndValues ...any) {
	z.sugaredLogger.Infow(msg, keysAndValues...)
}

// Warn logs a message at WarnLevel.
func (z *ZapLogger) Warn(msg string, keysAndValues ...any) {
	z.sugaredLogger.Warnw(msg, keysAndValues...)
}

// Error logs a message at ErrorLevel.
func (z *ZapLogger) Error(msg string, keysAndValues ...any) {
	z.sugaredLogger.Errorw(msg, keysAndValues...)
}

func (z *ZapLogger) Fatal(msg string, keysAndValues ...any) {
	z.sugaredLogger.Fatalw(msg, keysAndValues...)
}

func (z *ZapLogger) Inner() *zap.SugaredLogger {
	return z.sugaredLogger
}
