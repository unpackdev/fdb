// networking/discovery_metrics.go

package networking

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// DiscoveryMetrics holds all the metrics instruments for the DiscoveryService.
type DiscoveryMetrics struct {
	PeersAdvertisedTotal       metric.Int64Counter
	PeersAdvertisementFailures metric.Int64Counter
	PeersDiscoveredTotal       metric.Int64Counter
	PeersDiscoveryFailures     metric.Int64Counter
	DiscoveryLatencySeconds    metric.Float64Histogram
	BootstrapPeersConnected    metric.Int64Counter
	BootstrapPeersFailed       metric.Int64Counter
	ActivePeers                metric.Int64UpDownCounter
	PeersRemovedTotal          metric.Int64Counter
}

// InitializeDiscoveryMetrics initializes the metrics instruments.
func InitializeDiscoveryMetrics(ctx context.Context, meter metric.Meter) (*DiscoveryMetrics, error) {
	m := &DiscoveryMetrics{}
	var err error

	// Peer Discovery Metrics
	m.PeersAdvertisedTotal, err = meter.Int64Counter("networking.discovery.peers_advertised_total",
		metric.WithDescription("Total number of services advertised"),
	)
	if err != nil {
		return nil, err
	}

	m.PeersAdvertisementFailures, err = meter.Int64Counter("networking.discovery.peers_advertisement_failures",
		metric.WithDescription("Total number of failed service advertisements"),
	)
	if err != nil {
		return nil, err
	}

	m.PeersDiscoveredTotal, err = meter.Int64Counter("networking.discovery.peers_discovered_total",
		metric.WithDescription("Total number of peers discovered"),
	)
	if err != nil {
		return nil, err
	}

	m.PeersDiscoveryFailures, err = meter.Int64Counter("networking.discovery.peers_discovery_failures",
		metric.WithDescription("Total number of failed peer discovery attempts"),
	)
	if err != nil {
		return nil, err
	}

	m.DiscoveryLatencySeconds, err = meter.Float64Histogram("networking.discovery.latency_seconds",
		metric.WithDescription("Latency for peer discovery operations in seconds"),
	)
	if err != nil {
		return nil, err
	}

	// Bootstrap Metrics
	m.BootstrapPeersConnected, err = meter.Int64Counter("networking.bootstrap.peers_connected_total",
		metric.WithDescription("Total number of successfully connected bootstrap peers"),
	)
	if err != nil {
		return nil, err
	}

	m.BootstrapPeersFailed, err = meter.Int64Counter("networking.bootstrap.peers_failed_total",
		metric.WithDescription("Total number of failed bootstrap peer connections"),
	)
	if err != nil {
		return nil, err
	}

	// General Metrics
	m.ActivePeers, err = meter.Int64UpDownCounter("networking.discovery.active_peers",
		metric.WithDescription("Current number of active peers in the routing table"),
	)
	if err != nil {
		return nil, err
	}

	m.PeersRemovedTotal, err = meter.Int64Counter("networking.discovery.peers_removed_total",
		metric.WithDescription("Total number of peers removed from the routing table"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordPeersAdvertised increments the PeersAdvertisedTotal counter.
func (m *DiscoveryMetrics) RecordPeersAdvertised(ctx context.Context, count int64) {
	m.PeersAdvertisedTotal.Add(ctx, count)
}

// RecordPeersAdvertisementFailure increments the PeersAdvertisementFailures counter.
func (m *DiscoveryMetrics) RecordPeersAdvertisementFailure(ctx context.Context, count int64) {
	m.PeersAdvertisementFailures.Add(ctx, count)
}

// RecordPeersDiscovered increments the PeersDiscoveredTotal counter.
func (m *DiscoveryMetrics) RecordPeersDiscovered(ctx context.Context, count int64) {
	m.PeersDiscoveredTotal.Add(ctx, count)
}

// RecordPeersDiscoveryFailure increments the PeersDiscoveryFailures counter.
func (m *DiscoveryMetrics) RecordPeersDiscoveryFailure(ctx context.Context, count int64) {
	m.PeersDiscoveryFailures.Add(ctx, count)
}

// RecordDiscoveryLatency records the latency of a discovery operation.
func (m *DiscoveryMetrics) RecordDiscoveryLatency(ctx context.Context, latency time.Duration) {
	m.DiscoveryLatencySeconds.Record(ctx, latency.Seconds())
}

// RecordBootstrapPeersConnected increments the BootstrapPeersConnected counter.
func (m *DiscoveryMetrics) RecordBootstrapPeersConnected(ctx context.Context, count int64) {
	m.BootstrapPeersConnected.Add(ctx, count)
}

// RecordBootstrapPeersFailed increments the BootstrapPeersFailed counter.
func (m *DiscoveryMetrics) RecordBootstrapPeersFailed(ctx context.Context, count int64) {
	m.BootstrapPeersFailed.Add(ctx, count)
}

// RecordActivePeers updates the current number of active peers.
// The delta can be positive (peer added) or negative (peer removed).
func (m *DiscoveryMetrics) RecordActivePeers(ctx context.Context, delta int64) {
	m.ActivePeers.Add(ctx, delta)
}

// RecordPeersRemoved increments the PeersRemovedTotal counter.
func (m *DiscoveryMetrics) RecordPeersRemoved(ctx context.Context, count int64) {
	m.PeersRemovedTotal.Add(ctx, count)
}
