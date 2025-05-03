// pkg/share/collector.go

package share

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Collector defines the interface for collecting and updating utility metrics for peers.
//
// Implementations of this interface are responsible for managing metrics associated with peers,
// providing methods to update individual metrics, compute utility scores, and retrieve metrics data.
type Collector interface {
	// UpdateResponsiveness updates the responsiveness metric for a peer.
	//
	// Parameters:
	//   - p: The peer ID of the peer being updated.
	//   - latency: The latency observed in communication with the peer.
	//   - success: Indicates whether the communication attempt was successful.
	UpdateResponsiveness(p peer.ID, latency float64, success bool)

	// UpdateReliability updates the reliability metric for a peer.
	//
	// Parameters:
	//   - p: The peer ID of the peer being updated.
	//   - success: Indicates whether the communication attempt was successful.
	UpdateReliability(p peer.ID, success bool)

	// UpdateBandwidthUsage updates the bandwidth usage metric for a peer.
	//
	// Parameters:
	//   - p: The peer ID of the peer being updated.
	//   - value: The new bandwidth usage value.
	UpdateBandwidthUsage(p peer.ID, value float64)

	// UpdateComputational updates the computational metric for a peer.
	//
	// Parameters:
	//   - p: The peer ID of the peer being updated.
	//   - value: The new computational metric value.
	UpdateComputational(p peer.ID, value float64)

	// UpdateStorage updates the storage metric for a peer.
	//
	// Parameters:
	//   - p: The peer ID of the peer being updated.
	//   - value: The new storage metric value.
	UpdateStorage(p peer.ID, value float64)

	// UpdateUptime updates the uptime metric for a peer.
	//
	// Parameters:
	//   - p: The peer ID of the peer being updated.
	//   - value: The new uptime metric value.
	UpdateUptime(p peer.ID, value float64)

	// CalculateUtilityScore computes the overall utility score for a peer.
	//
	// Parameters:
	//   - p: The peer ID of the peer for which the utility score is computed.
	//
	// Returns:
	//   - The computed utility score as a float64.
	CalculateUtilityScore(p peer.ID) float64

	// GetMetrics retrieves the metrics for a given peer.
	//
	// Parameters:
	//   - p: The peer ID of the peer whose metrics are retrieved.
	//
	// Returns:
	//   - A pointer to the Metrics struct containing the peer's metrics.
	GetMetrics(p peer.ID) *Metrics

	// GetAllMetrics returns a copy of all collected metrics.
	//
	// Returns:
	//   - A map of peer IDs to their corresponding Metrics structs.
	GetAllMetrics() map[peer.ID]*Metrics

	// RemoveOldMetrics removes metrics that haven't been updated within the specified threshold.
	//
	// Parameters:
	//   - threshold: A duration specifying the age threshold for removing old metrics.
	RemoveOldMetrics(threshold time.Duration)

	// AdjustEmaAlpha dynamically adjusts the smoothing factor used in Exponential Moving Average calculations.
	//
	// Parameters:
	//   - newAlpha: The new EMA alpha value to be set.
	AdjustEmaAlpha(newAlpha float64)
}
