// pkg/share/metrics.go

package share

import (
	"github.com/holiman/uint256"
	"time"
)

// Metrics holds various performance metrics for a peer.
//
// Fields:
// - BandwidthUsage: Represents the bandwidth usage metric for the peer.
// - Computational: Represents the computational power metric for the peer.
// - Storage: Represents the storage capacity metric for the peer.
// - Uptime: Represents the uptime percentage metric for the peer.
// - Responsiveness: Represents the responsiveness metric, typically calculated based on latency.
// - Reliability: Represents the reliability metric, possibly based on successful interactions.
// - Stake: Represents the stake or trust level associated with the peer.
// - LastUpdated: Timestamp of the last update to the metrics.
type Metrics struct {
	BandwidthUsage float64      // Bandwidth usage metric
	Computational  float64      // Computational power metric
	Storage        float64      // Storage capacity metric
	Uptime         float64      // Uptime percentage metric
	Responsiveness float64      // Responsiveness metric based on latency
	Reliability    float64      // Reliability metric based on successful interactions
	Stake          *uint256.Int // Stake or trust level associated with the peer
	LastUpdated    time.Time    // Timestamp of the last update
}
