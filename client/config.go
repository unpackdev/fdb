package client

// Config holds the configuration for the Client, including transports
type Config struct {
	Transports map[string]Transport
}

// NewConfig creates and initializes a Config instance
func NewConfig() *Config {
	return &Config{
		Transports: make(map[string]Transport),
	}
}
