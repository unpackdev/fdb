package config

type Logger struct {
	Enabled     bool   `yaml:"enabled"`
	Environment string `yaml:"environment"`
	Level       string `yaml:"level"`
}
