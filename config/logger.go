package config

// Logger represents the configuration for the logging system used by the application.
// It allows customization of logging behavior, including enabling/disabling logging,
// setting the environment (e.g., production, development), and specifying the log level.
type Logger struct {
	// Enabled determines whether logging is enabled or disabled in the application.
	// If set to false, logging will be disabled.
	Enabled bool `yaml:"enabled"`

	// Environment defines the environment in which the application is running (e.g., "production" or "development").
	// This field can be used to adjust logging behavior based on the environment (e.g., verbose logging in development, minimal logging in production).
	Environment string `yaml:"environment"`

	// Level specifies the log level (e.g., "debug", "info", "warn", "error"). The log level controls
	// the verbosity of the log output, with higher levels (like "error") showing only critical messages.
	Level string `yaml:"level"`
}
