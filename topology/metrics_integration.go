// pkg/topology/metrics_integration.go
package topology

import (
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/metrics"
	"go.opentelemetry.io/otel/metric"
)

// InitializeMetrics initializes all topology metrics using the provided meter.
func InitializeMetrics(meter metric.Meter) (*TopologyMetrics, error) {
	// Initialize topology metrics using the meter
	return InitializeTopologyMetrics(meter)
}

// integrateTopologyMetrics integrates the TopologyMetrics into the Topology struct.
// This function should be called when initializing a Topology instance.
func integrateTopologyMetrics(t *Topology, _ *metrics.Collector) error {
	// Get the observability from the network since we don't have direct access to it
	if t.network == nil {
		return errors.New("network not initialized")
	}

	obs := t.network.Observability
	if obs == nil {
		return errors.New("observability not initialized in network")
	}

	// Initialize topology metrics with the meter from observability
	topologyMetrics, err := InitializeMetrics(obs.Meter)
	if err != nil {
		return err
	}

	// Store the metrics in the actors instance for access by other methods
	if t.actors != nil {
		t.actors.topologyMetrics = topologyMetrics
	}

	return nil
}
