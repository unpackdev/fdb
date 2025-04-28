package config

import "time"

// Observability represents the configuration for the observability package,
// including metrics and tracing settings.
type Observability struct {
	// MetricsConfig holds the configuration for metrics collection and exporting.
	Metrics MetricsConfig `yaml:"metrics"`

	// TracingConfig holds the configuration for tracing collection and exporting.
	Tracing TracingConfig `yaml:"tracing"`
}

// MetricsConfig holds the configuration settings for metrics.
type MetricsConfig struct {
	Enable         bool              `yaml:"enable"`
	Exporter       string            `yaml:"exporter"` // e.g., "otlp", "prometheus"
	Endpoint       string            `yaml:"endpoint"`
	Headers        map[string]string `yaml:"headers"`
	ExportInterval time.Duration     `yaml:"export_interval"`
	SampleRate     float64           `yaml:"sample_rate"` // For sampling strategies if applicable
}

// TracingConfig holds the configuration settings for tracing.
type TracingConfig struct {
	Enable         bool              `yaml:"enable"`
	Exporter       string            `yaml:"exporter"` // e.g., "otlp", "jaeger"
	Endpoint       string            `yaml:"endpoint"`
	Headers        map[string]string `yaml:"headers"`
	Sampler        string            `yaml:"sampler"`       // e.g., "always_on", "probability"
	SamplingRate   float64           `yaml:"sampling_rate"` // Used if Sampler is "probability"
	ExportInterval time.Duration     `yaml:"export_interval"`
}
