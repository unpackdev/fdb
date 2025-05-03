// pkg/metrics/collector.go

package metrics

import (
	"context"

	"github.com/holiman/uint256"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/share"
	"github.com/unpackdev/fdb/pkg/types"

	"math/big"
	"time"

	"github.com/sasha-s/go-deadlock"
	"go.uber.org/zap"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Ensure that Collector implements the share.Collector interface
var _ share.Collector = (*Collector)(nil)

// Collector collects and updates utility metrics for peers.
//
// Fields:
// - metrics: A map of peer IDs to their associated Metrics.
// - mu: A read-write mutex for synchronizing access to metrics.
// - weights: Weights assigned to different metrics for utility score calculation.
// - emaAlpha: Smoothing factor used in Exponential Moving Average calculations.
// - maxRespScore: Maximum cap for responsiveness to prevent extreme values.
type Collector struct {
	ctx            context.Context
	logger         logger.Logger
	metrics        map[peer.ID]*share.Metrics
	mu             deadlock.RWMutex
	weights        share.Metrics
	emaAlpha       float64                   // Smoothing factor for EMA
	maxRespScore   float64                   // Maximum cap for responsiveness
	peerAddressMap map[peer.ID]types.Address // Map peer.ID to types.Address

}

// NewCollector initializes a new Collector with the provided weights and EMA alpha.
//
// Parameters:
// - weights: Weights for each metric used in utility score calculation.
// - emaAlpha: Smoothing factor for Exponential Moving Average calculations.
// - maxRespScore: Maximum cap for responsiveness to prevent extreme values.
//
// Returns:
// - A pointer to the initialized Collector.
func NewCollector(ctx context.Context, logger logger.Logger, weights share.Metrics, emaAlpha float64, maxRespScore float64) *Collector {
	toReturn := &Collector{
		ctx:            ctx,
		logger:         logger,
		metrics:        make(map[peer.ID]*share.Metrics),
		weights:        weights,
		emaAlpha:       emaAlpha,
		maxRespScore:   maxRespScore,
		peerAddressMap: make(map[peer.ID]types.Address),
	}

	return toReturn
}

// UpdateResponsiveness updates the responsiveness metric for a peer.
//
// Parameters:
// - p: The peer ID of the peer being updated.
// - latency: The latency observed in communication with the peer.
// - success: Indicates whether the communication attempt was successful.
func (mc *Collector) UpdateResponsiveness(p peer.ID, latency float64, success bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)

	metrics := mc.metrics[p]

	if success && latency > 0 {
		normalizedResponsiveness := 1.0 / latency // Higher score for lower latency

		// Cap the normalized responsiveness to prevent extreme values
		if normalizedResponsiveness > mc.maxRespScore {
			normalizedResponsiveness = mc.maxRespScore
		}

		metrics.Responsiveness = mc.emaAlpha*normalizedResponsiveness + (1-mc.emaAlpha)*metrics.Responsiveness
	} else {
		// Penalize responsiveness on failure
		metrics.Responsiveness = (1 - mc.emaAlpha) * metrics.Responsiveness
	}

	metrics.LastUpdated = time.Now()
}

// UpdateReliability updates the reliability metric for a peer.
//
// Parameters:
// - p: The peer ID of the peer being updated.
// - success: Indicates whether the communication attempt was successful.
func (mc *Collector) UpdateReliability(p peer.ID, success bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)

	metrics := mc.metrics[p]

	if success {
		metrics.Reliability = mc.emaAlpha*1.0 + (1-mc.emaAlpha)*metrics.Reliability
	} else {
		metrics.Reliability = (1 - mc.emaAlpha) * metrics.Reliability
	}

	metrics.LastUpdated = time.Now()
}

// UpdateBandwidthUsage updates the bandwidth usage metric for a peer.
//
// Parameters:
// - p: The peer ID of the peer being updated.
// - value: The new bandwidth usage value.
func (mc *Collector) UpdateBandwidthUsage(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].BandwidthUsage = value
	mc.metrics[p].LastUpdated = time.Now()
}

// UpdateComputational updates the computational metric for a peer.
//
// Parameters:
// - p: The peer ID of the peer being updated.
// - value: The new computational metric value.
func (mc *Collector) UpdateComputational(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].Computational = value
	mc.metrics[p].LastUpdated = time.Now()
}

// UpdateStorage updates the storage metric for a peer.
//
// Parameters:
// - p: The peer ID of the peer being updated.
// - value: The new storage metric value.
func (mc *Collector) UpdateStorage(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].Storage = value
	mc.metrics[p].LastUpdated = time.Now()
}

// UpdateUptime updates the uptime metric for a peer.
//
// Parameters:
// - p: The peer ID of the peer being updated.
// - value: The new uptime metric value.
func (mc *Collector) UpdateUptime(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].Uptime = value
	mc.metrics[p].LastUpdated = time.Now()
}

// CalculateUtilityScore computes the overall utility score for a peer.
//
// Parameters:
// - p: The peer ID of the peer for which the utility score is computed.
//
// Returns:
// - The computed utility score as a float64.
func (mc *Collector) CalculateUtilityScore(p peer.ID) float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	m, exists := mc.metrics[p]
	if !exists {
		return 0.0
	}
	w := mc.weights

	var stakeFloat float64
	if m.Stake != nil {
		stakeFloat, _ = new(big.Float).SetInt(m.Stake.ToBig()).Float64()
	} else {
		stakeFloat = 0.0
	}

	utilityScore := (m.BandwidthUsage * w.BandwidthUsage) +
		(m.Computational * w.Computational) +
		(m.Storage * w.Storage) +
		(m.Uptime * w.Uptime) +
		(m.Responsiveness * w.Responsiveness) +
		(m.Reliability * w.Reliability) +
		stakeFloat

	return utilityScore
}

// GetMetrics retrieves the metrics for a given peer.
//
// Parameters:
// - p: The peer ID of the peer whose metrics are retrieved.
//
// Returns:
// - A pointer to the Metrics struct containing the peer's metrics.
func (mc *Collector) GetMetrics(p peer.ID) *share.Metrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	if m, exists := mc.metrics[p]; exists {
		return m
	}
	return nil
}

// GetAllMetrics returns a copy of all collected metrics.
//
// Returns:
// - A map of peer IDs to their corresponding Metrics structs.
func (mc *Collector) GetAllMetrics() map[peer.ID]*share.Metrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	// Create a copy to avoid race conditions
	metricsCopy := make(map[peer.ID]*share.Metrics)
	for p, m := range mc.metrics {
		// Deep copy the metrics
		metricsCopy[p] = &share.Metrics{
			BandwidthUsage: m.BandwidthUsage,
			Computational:  m.Computational,
			Storage:        m.Storage,
			Uptime:         m.Uptime,
			Responsiveness: m.Responsiveness,
			Reliability:    m.Reliability,
			Stake:          m.Stake,
			LastUpdated:    m.LastUpdated,
		}
	}
	return metricsCopy
}

// RemoveOldMetrics removes metrics that haven't been updated within the specified threshold.
//
// Parameters:
// - threshold: A duration specifying the age threshold for removing old metrics.
func (mc *Collector) RemoveOldMetrics(threshold time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	now := time.Now()
	for p, metrics := range mc.metrics {
		if now.Sub(metrics.LastUpdated) > threshold {
			delete(mc.metrics, p)
		}
	}
}

// AdjustEmaAlpha dynamically adjusts the smoothing factor used in EMA calculations.
//
// Parameters:
// - newAlpha: The new EMA alpha value to be set.
func (mc *Collector) AdjustEmaAlpha(newAlpha float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.emaAlpha = newAlpha
}

func (mc *Collector) MapPeerToAddress(p peer.ID, address types.Address) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.peerAddressMap[p] = address
}

// ensurePeer ensures that the peer has metrics initialized.
// It locks the mutex before calling ensurePeerLocked.
func (mc *Collector) ensurePeer(p peer.ID) {
	mc.ensurePeerLocked(p)
}

// ensurePeerLocked initializes metrics for a peer if not already present.
// Note: This method assumes that mc.mu is already locked.
func (mc *Collector) ensurePeerLocked(p peer.ID) {
	if _, exists := mc.metrics[p]; !exists {
		// Fetch the stake amount
		var stake *uint256.Int

		mc.metrics[p] = &share.Metrics{
			BandwidthUsage: 0.0,
			Computational:  0.0,
			Storage:        0.0,
			Uptime:         0.0,
			Responsiveness: 0.0,
			Reliability:    0.0,
			Stake:          stake,
			LastUpdated:    time.Now(),
		}
		mc.logger.Info("Initialized metrics for peer", zap.String("peer_id", p.String()))
	}
}
