package metrics

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Metrics acts as an OTel metrics manager.
type Metrics struct {
	meterProvider *sdkmetric.MeterProvider
}

// NewMetrics initializes the OTel metric exporter and meter provider.
func NewMetrics(ctx context.Context, endpoint string) (*Metrics, error) {
	// Configure the OTLP exporter to send metrics to the OTel collector.
	exporter, err := otlpmetric.New(
		ctx,
		otlpmetricgrpc.NewClient(
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		),
	)
	if err != nil {
		return nil, err
	}

	// Set up a periodic reader to batch and export metrics at intervals.
	reader := sdkmetric.NewPeriodicReader(
		exporter,
		sdkmetric.WithInterval(15*time.Second),
	)

	// Create a new MeterProvider with the periodic reader.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)

	// Set the global meter provider to ensure consistency across the application.
	otel.SetMeterProvider(provider)

	return &Metrics{
		meterProvider: provider,
	}, nil
}

// GetMeter returns a named meter for instrumentation.
func (m *Metrics) GetMeter(instrumentationName string) metric.Meter {
	return m.meterProvider.Meter(instrumentationName)
}

// Shutdown gracefully shuts down the meter provider and flushes any remaining metrics.
func (m *Metrics) Shutdown(ctx context.Context) error {
	return m.meterProvider.Shutdown(ctx)
}
