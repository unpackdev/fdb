package observability

import (
	"context"

	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const ServiceName = "fdb"

// Observability encapsulates metrics, tracing, and logging.
type Observability struct {
	Logger         logger.Logger
	Meter          metric.Meter
	Tracer         trace.Tracer
	TracerProvider *sdktrace.TracerProvider
	ServiceName    string
}

// New initializes all observability components based on the provided configuration.
func New(ctx context.Context, cfg config.Config, logger logger.Logger) (*Observability, error) {
	var obs Observability
	var err error

	obs.Logger = logger
	obs.ServiceName = ServiceName

	// Initialize Metrics
	if cfg.Observability.Metrics.Enable {

		obs.Meter, err = InitMetrics(ctx, cfg.Id, cfg.Observability.Metrics, logger)
		if err != nil {
			obs.Logger.Error("Failed to initialize metrics", zap.Error(err))
			return nil, err
		}

		// Initialize metrics instruments
		err = InitializeMetricsInObservability(ctx, obs.Meter, logger)
		if err != nil {
			obs.Logger.Error("Failed to initialize metrics instruments", zap.Error(err))
			return nil, err
		}
	} else {
		obs.Meter = otel.Meter("disabled")
	}

	// Initialize Tracing
	if cfg.Observability.Tracing.Enable {
		obs.Tracer, obs.TracerProvider, err = InitTracer(ctx, cfg.Id, cfg.Observability.Tracing, logger)
		if err != nil {
			obs.Logger.Error("Failed to initialize tracer", zap.Error(err))
			return nil, err
		}
	} else {
		obs.Tracer = otel.Tracer("disabled")
	}

	return &obs, nil
}

// Shutdown gracefully shuts down the observability providers.
func (o *Observability) Shutdown(ctx context.Context) error {
	var err error
	if o.TracerProvider != nil {
		if shutdownErr := o.TracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = shutdownErr
		}
	}
	return err
}

// RecordServiceStateChange records a service state change event.
func (o *Observability) RecordServiceStateChange(ctx context.Context, serviceType, state string) {
	if ServiceStateCounter != nil {
		ServiceStateCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("service_type", serviceType),
			attribute.String("state", state),
		))
	}
}

// EmitServiceStateChangeTrace emits a trace when a service state changes.
func (o *Observability) EmitServiceStateChangeTrace(ctx context.Context, serviceType, previousState, newState string) {
	_, span := o.Tracer.Start(ctx, "ServiceStateChange")
	span.SetAttributes(
		attribute.String("service_type", serviceType),
		attribute.String("previous_state", previousState),
		attribute.String("new_state", newState),
	)
	span.End()
}
