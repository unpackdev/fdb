// pkg/topology/metrics.go
package topology

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// TopologyMetrics holds all the metrics instruments for the Topology management.
type TopologyMetrics struct {
	// Actor management metrics
	ActorsAddedTotal          metric.Int64Counter
	ActorsRemovedTotal        metric.Int64Counter
	ActorVerificationsTotal   metric.Int64Counter
	ActorVerificationFailures metric.Int64Counter
	ActorVerificationLatency  metric.Float64Histogram

	// Pending peer metrics
	PendingPeersTotal        metric.Int64Counter
	PendingPeersTimeoutTotal metric.Int64Counter

	// Role-based metrics
	ActorsByRoleCount metric.Int64UpDownCounter

	// Connection metrics
	ActorConnectionsTotal    metric.Int64Counter
	ActorDisconnectionsTotal metric.Int64Counter

	// Consensus-related metrics
	ConsensusActorsTotal metric.Int64UpDownCounter

	// Health metrics
	TopologyChangeLatency      metric.Float64Histogram
	// Wait operation metrics
	WaitOperationsTotal        metric.Int64Counter
	WaitOperationSuccess       metric.Int64Counter
	WaitOperationTimeout       metric.Int64Counter
	WaitOperationLatency       metric.Float64Histogram
}

// InitializeTopologyMetrics initializes the metrics instruments for Topology.
func InitializeTopologyMetrics(meter metric.Meter) (*TopologyMetrics, error) {
	m := &TopologyMetrics{}
	var err error

	// Initialize Actor Management Metrics
	m.ActorsAddedTotal, err = meter.Int64Counter(
		"topology.actors_added_total",
		metric.WithDescription("Total number of actors added to the topology"),
	)
	if err != nil {
		return nil, err
	}

	m.ActorsRemovedTotal, err = meter.Int64Counter(
		"topology.actors_removed_total",
		metric.WithDescription("Total number of actors removed from the topology"),
	)
	if err != nil {
		return nil, err
	}

	m.ActorVerificationsTotal, err = meter.Int64Counter(
		"topology.actor_verifications_total",
		metric.WithDescription("Total number of actor verification attempts"),
	)
	if err != nil {
		return nil, err
	}

	m.ActorVerificationFailures, err = meter.Int64Counter(
		"topology.actor_verification_failures_total",
		metric.WithDescription("Total number of actor verification failures"),
	)
	if err != nil {
		return nil, err
	}

	m.ActorVerificationLatency, err = meter.Float64Histogram(
		"topology.actor_verification_latency_seconds",
		metric.WithDescription("Latency of actor verification operations in seconds"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize Pending Peer Metrics
	m.PendingPeersTotal, err = meter.Int64Counter(
		"topology.pending_peers_total",
		metric.WithDescription("Total number of pending peers waiting for verification"),
	)
	if err != nil {
		return nil, err
	}

	m.PendingPeersTimeoutTotal, err = meter.Int64Counter(
		"topology.pending_peers_timeout_total",
		metric.WithDescription("Total number of pending peers that timed out before verification"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize Role-based Metrics
	m.ActorsByRoleCount, err = meter.Int64UpDownCounter(
		"topology.actors_by_role_count",
		metric.WithDescription("Number of actors currently in the topology by role"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize Connection Metrics
	m.ActorConnectionsTotal, err = meter.Int64Counter(
		"topology.actor_connections_total",
		metric.WithDescription("Total number of actor connections established"),
	)
	if err != nil {
		return nil, err
	}

	m.ActorDisconnectionsTotal, err = meter.Int64Counter(
		"topology.actor_disconnections_total",
		metric.WithDescription("Total number of actor disconnections"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize Consensus Metrics
	m.ConsensusActorsTotal, err = meter.Int64UpDownCounter(
		"topology.consensus_actors_total",
		metric.WithDescription("Number of actors participating in consensus"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize Health Metrics
	m.TopologyChangeLatency, err = meter.Float64Histogram(
		"topology.change_latency_seconds",
		metric.WithDescription("Latency of topology change operations in seconds"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize Wait Operation Metrics
	m.WaitOperationsTotal, err = meter.Int64Counter(
		"topology.wait_operations_total",
		metric.WithDescription("Total number of wait operations for peers"),
	)
	if err != nil {
		return nil, err
	}

	m.WaitOperationSuccess, err = meter.Int64Counter(
		"topology.wait_operation_success_total",
		metric.WithDescription("Total number of successful wait operations for peers"),
	)
	if err != nil {
		return nil, err
	}

	m.WaitOperationTimeout, err = meter.Int64Counter(
		"topology.wait_operation_timeout_total",
		metric.WithDescription("Total number of wait operations that timed out"),
	)
	if err != nil {
		return nil, err
	}

	m.WaitOperationLatency, err = meter.Float64Histogram(
		"topology.wait_operation_latency_seconds",
		metric.WithDescription("Latency of wait operations in seconds"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordActorAdded increments the ActorsAddedTotal counter.
func (m *TopologyMetrics) RecordActorAdded(ctx context.Context, count int64, role string) {
	attrs := []attribute.KeyValue{attribute.String("role", role)}
	m.ActorsAddedTotal.Add(ctx, count)
	m.ActorsByRoleCount.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordActorRemoved increments the ActorsRemovedTotal counter.
func (m *TopologyMetrics) RecordActorRemoved(ctx context.Context, count int64, role string) {
	attrs := []attribute.KeyValue{attribute.String("role", role)}
	m.ActorsRemovedTotal.Add(ctx, count)
	m.ActorsByRoleCount.Add(ctx, -count, metric.WithAttributes(attrs...))
}

// RecordActorVerification increments the ActorVerificationsTotal counter.
func (m *TopologyMetrics) RecordActorVerification(ctx context.Context, count int64) {
	m.ActorVerificationsTotal.Add(ctx, count)
}

// RecordActorVerificationFailure increments the ActorVerificationFailures counter.
func (m *TopologyMetrics) RecordActorVerificationFailure(ctx context.Context, count int64) {
	m.ActorVerificationFailures.Add(ctx, count)
}

// RecordActorVerificationLatency records the latency of actor verification operations.
func (m *TopologyMetrics) RecordActorVerificationLatency(ctx context.Context, duration time.Duration) {
	m.ActorVerificationLatency.Record(ctx, duration.Seconds())
}

// RecordPendingPeer increments the PendingPeersTotal counter.
func (m *TopologyMetrics) RecordPendingPeer(ctx context.Context, count int64) {
	m.PendingPeersTotal.Add(ctx, count)
}

// RecordPendingPeerTimeout increments the PendingPeersTimeoutTotal counter.
func (m *TopologyMetrics) RecordPendingPeerTimeout(ctx context.Context, count int64) {
	m.PendingPeersTimeoutTotal.Add(ctx, count)
}

// RecordActorConnection increments the ActorConnectionsTotal counter.
func (m *TopologyMetrics) RecordActorConnection(ctx context.Context, count int64) {
	m.ActorConnectionsTotal.Add(ctx, count)
}

// RecordActorDisconnection increments the ActorDisconnectionsTotal counter.
func (m *TopologyMetrics) RecordActorDisconnection(ctx context.Context, count int64) {
	m.ActorDisconnectionsTotal.Add(ctx, count)
}

// RecordConsensusActor updates the ConsensusActorsTotal counter.
func (m *TopologyMetrics) RecordConsensusActor(ctx context.Context, delta int64) {
	m.ConsensusActorsTotal.Add(ctx, delta)
}

// RecordTopologyChangeLatency records the latency of topology change operations.
func (m *TopologyMetrics) RecordTopologyChangeLatency(ctx context.Context, duration time.Duration) {
	m.TopologyChangeLatency.Record(ctx, duration.Seconds())
}

// RecordWaitOperation increments the WaitOperationsTotal counter.
func (m *TopologyMetrics) RecordWaitOperation(ctx context.Context, count int64, waitType string) {
	attrs := []attribute.KeyValue{attribute.String("wait_type", waitType)}
	m.WaitOperationsTotal.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordWaitOperationSuccess increments the WaitOperationSuccess counter.
func (m *TopologyMetrics) RecordWaitOperationSuccess(ctx context.Context, count int64, waitType string) {
	attrs := []attribute.KeyValue{attribute.String("wait_type", waitType)}
	m.WaitOperationSuccess.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordWaitOperationTimeout increments the WaitOperationTimeout counter.
func (m *TopologyMetrics) RecordWaitOperationTimeout(ctx context.Context, count int64, waitType string) {
	attrs := []attribute.KeyValue{attribute.String("wait_type", waitType)}
	m.WaitOperationTimeout.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordWaitOperationLatency records the latency of wait operations.
func (m *TopologyMetrics) RecordWaitOperationLatency(ctx context.Context, duration time.Duration, waitType string) {
	attrs := []attribute.KeyValue{attribute.String("wait_type", waitType)}
	m.WaitOperationLatency.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}
