package logger

// noOpLogger is a Logger implementation that does nothing.
// Useful as a fallback when logging is disabled or not set.
type noOpLogger struct{}

// NewNoOpLogger creates a new instance of noOpLogger.
func NewNoOpLogger() Logger {
	return &noOpLogger{}
}

func (l *noOpLogger) Debug(msg string, keysAndValues ...any) {}
func (l *noOpLogger) Info(msg string, keysAndValues ...any)  {}
func (l *noOpLogger) Warn(msg string, keysAndValues ...any)  {}
func (l *noOpLogger) Error(msg string, keysAndValues ...any) {}
func (l *noOpLogger) Fatal(msg string, keysAndValues ...any) {}
