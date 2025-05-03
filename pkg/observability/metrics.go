package observability

import (
	"context"

	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.uber.org/zap"
)

// ServiceStateCounter is a counter for service state changes.
var ServiceStateCounter metric.Int64Counter

// InitMetrics initializes OpenTelemetry metrics.
func InitMetrics(ctx context.Context, cfg config.MetricsConfig, logger logger.Logger) (metric.Meter, error) {
	if !cfg.Enable {
		return otel.Meter("disabled"), nil
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
		otlpmetricgrpc.WithHeaders(cfg.Headers),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		logger.Error("Failed to create OTLP metric exporter", zap.Error(err))
		return nil, err
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(ServiceName),
		),
	)
	if err != nil {
		logger.Error("Failed to create resource for metrics", zap.Error(err))
		return nil, err
	}

	// Create a PeriodicReader
	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.ExportInterval))

	// Create MeterProvider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)

	otel.SetMeterProvider(meterProvider)
	meter := otel.Meter(ServiceName)

	// Initialize the service state counter
	ServiceStateCounter, err = meter.Int64Counter("service_state_changes",
		metric.WithDescription("Counts the number of service state changes"),
	)
	if err != nil {
		logger.Error("Failed to create service_state_changes counter", zap.Error(err))
		return nil, err
	}

	return meter, nil
}

// InitializeMetricsInObservability initializes any additional metrics instruments.
func InitializeMetricsInObservability(ctx context.Context, meter metric.Meter, logger logger.Logger) error {
	// You can initialize additional metrics instruments here if needed.
	return nil
}
