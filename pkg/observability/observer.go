package observability

import (
	"context"
	"go.opentelemetry.io/otel/trace"
)

// Observer defines the interface for observability capabilities
type Observer interface {
	// StartSpan starts a new span and returns the new context and span
	StartSpan(ctx context.Context, name string) (context.Context, trace.Span)
}

// Ensure Observability implements the Observer interface
var _ Observer = (*Observability)(nil)

// StartSpan starts a new span and returns the new context and span
func (o *Observability) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	if o == nil || o.Tracer == nil {
		// Return no-op span if observability is not configured
		return ctx, trace.SpanFromContext(ctx)
	}
	return o.Tracer.Start(ctx, name)
}

// NoopObserver is a no-operation implementation of the Observer interface
type NoopObserver struct{}

// StartSpan implements the Observer interface but does nothing
func (n *NoopObserver) StartSpan(ctx context.Context, _ string) (context.Context, trace.Span) {
	return ctx, trace.SpanFromContext(ctx)
}

// NewNoopObserver creates a new no-operation observer
func NewNoopObserver() Observer {
	return &NoopObserver{}
}
