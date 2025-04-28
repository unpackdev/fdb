package networking

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// P2PMetrics holds all the metrics instruments for the P2PNetwork.
type P2PMetrics struct {
	PeersConnectedTotal   metric.Int64Counter
	PeersConnectionFailed metric.Int64Counter
	MessagesSentTotal     metric.Int64Counter
	MessagesReceivedTotal metric.Int64Counter
	MessageLatencySeconds metric.Float64Histogram
	ActivePeers           metric.Int64UpDownCounter
	PeersRemovedTotal     metric.Int64Counter
}

// InitializeP2PMetrics initializes the metrics instruments for P2PNetwork.
func InitializeP2PMetrics(ctx context.Context, meter metric.Meter) (*P2PMetrics, error) {
	m := &P2PMetrics{}
	var err error

	// Peer Connection Metrics
	m.PeersConnectedTotal, err = meter.Int64Counter(
		"networking.p2p.peers_connected_total",
		metric.WithDescription("Total number of successfully connected peers"),
	)
	if err != nil {
		return nil, err
	}

	m.PeersConnectionFailed, err = meter.Int64Counter(
		"networking.p2p.peers_connection_failed_total",
		metric.WithDescription("Total number of failed peer connection attempts"),
	)
	if err != nil {
		return nil, err
	}

	// Message Metrics
	m.MessagesSentTotal, err = meter.Int64Counter(
		"networking.p2p.messages_sent_total",
		metric.WithDescription("Total number of messages sent"),
	)
	if err != nil {
		return nil, err
	}

	m.MessagesReceivedTotal, err = meter.Int64Counter(
		"networking.p2p.messages_received_total",
		metric.WithDescription("Total number of messages received"),
	)
	if err != nil {
		return nil, err
	}

	m.MessageLatencySeconds, err = meter.Float64Histogram(
		"networking.p2p.message_latency_seconds",
		metric.WithDescription("Latency of message processing in seconds"),
	)
	if err != nil {
		return nil, err
	}

	// Active Peers Metrics
	m.ActivePeers, err = meter.Int64UpDownCounter(
		"networking.p2p.active_peers",
		metric.WithDescription("Current number of active peers in the network"),
	)
	if err != nil {
		return nil, err
	}

	// Peers Removed Metrics
	m.PeersRemovedTotal, err = meter.Int64Counter(
		"networking.p2p.peers_removed_total",
		metric.WithDescription("Total number of peers removed from the network"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordPeersConnected increments the PeersConnectedTotal counter.
func (m *P2PMetrics) RecordPeersConnected(ctx context.Context, count int64) {
	m.PeersConnectedTotal.Add(ctx, count)
}

// RecordPeersConnectionFailed increments the PeersConnectionFailed counter.
func (m *P2PMetrics) RecordPeersConnectionFailed(ctx context.Context, count int64) {
	m.PeersConnectionFailed.Add(ctx, count)
}

// RecordMessagesSent increments the MessagesSentTotal counter.
func (m *P2PMetrics) RecordMessagesSent(ctx context.Context, count int64) {
	m.MessagesSentTotal.Add(ctx, count)
}

// RecordMessagesReceived increments the MessagesReceivedTotal counter.
func (m *P2PMetrics) RecordMessagesReceived(ctx context.Context, count int64) {
	m.MessagesReceivedTotal.Add(ctx, count)
}

// RecordMessageLatency records the latency of a message processing.
func (m *P2PMetrics) RecordMessageLatency(ctx context.Context, latency time.Duration) {
	m.MessageLatencySeconds.Record(ctx, latency.Seconds())
}

// RecordActivePeers updates the current number of active peers.
// The delta can be positive (peer added) or negative (peer removed).
func (m *P2PMetrics) RecordActivePeers(ctx context.Context, delta int64) {
	m.ActivePeers.Add(ctx, delta)
}

// RecordPeersRemoved increments the PeersRemovedTotal counter.
func (m *P2PMetrics) RecordPeersRemoved(ctx context.Context, count int64) {
	m.PeersRemovedTotal.Add(ctx, count)
}
