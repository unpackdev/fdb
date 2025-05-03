// pkg/metrics/performance_monitor.go

package metrics

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// PerformanceMonitor pings peers and collects performance metrics.
type PerformanceMonitor struct {
	host                  host.Host
	logger                logger.Logger
	obs                   *observability.Observability
	collector             *Collector
	pingInterval          time.Duration
	concurrency           int
	wg                    sync.WaitGroup
	ctx                   context.Context
	cancel                context.CancelFunc
	metricsTimeout        time.Duration
	cleanupTicker         *time.Ticker
	cleanupThreshold      time.Duration
	stakingUpdateInterval time.Duration
	maxRespScore          float64
}

// NewPerformanceMonitor initializes a new PerformanceMonitor.
func NewPerformanceMonitor(
	ctx context.Context,
	host host.Host,
	logger logger.Logger,
	obs *observability.Observability,
	collector *Collector,
	pingInterval time.Duration,
	concurrency int,
	metricsTimeout time.Duration,
	cleanupInterval time.Duration,
	cleanupThreshold time.Duration,
	stakingUpdateInterval time.Duration,
) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(ctx)

	// Register stream handler for ping/pong game based on which later on we are going to calculate
	// telemetry information.
	host.SetStreamHandler(PerformanceProtocolID, PongHandler(logger))

	return &PerformanceMonitor{
		ctx:                   ctx,
		cancel:                cancel,
		host:                  host,
		logger:                logger,
		obs:                   obs,
		collector:             collector,
		pingInterval:          pingInterval,
		concurrency:           concurrency,
		metricsTimeout:        metricsTimeout,
		cleanupTicker:         time.NewTicker(cleanupInterval),
		cleanupThreshold:      cleanupThreshold,
		stakingUpdateInterval: stakingUpdateInterval,
		maxRespScore:          10.0, // Example cap value
	}
}

// Start begins the peer monitoring and cleanup processes.
func (pm *PerformanceMonitor) Start() {
	pm.logger.Info("Starting P2P network performance monitor")
	peerChan := make(chan peer.ID, pm.concurrency)

	// Start the peer collection and pinging routine
	go func() {
		ticker := time.NewTicker(pm.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pm.ctx.Done():
				close(peerChan)
				return
			case <-ticker.C:
				peers := pm.host.Network().Peers()
				//pm.logger.Debug("Collected peers to ping", zap.Int("peer_count", len(peers)))
				for _, p := range peers {
					if p == pm.host.ID() {
						continue // Skip self
					}

					//pm.logger.Debug("Adding peer to ping channel", zap.String("peer_id", p.String()))
					select {
					case peerChan <- p:
					default:
						// Channel is full; skip adding more peers to prevent blocking
						pm.logger.Warn("Peer channel is full; skipping peer", zap.String("peer_id", p.String()))
					}
				}
			}
		}
	}()

	// Start worker pool
	for i := 0; i < pm.concurrency; i++ {
		pm.wg.Add(1)
		go pm.worker(peerChan)
	}

	// Start the cleanup routine
	go pm.cleanupRoutine()

}

// worker processes peers from the peer channel.
func (pm *PerformanceMonitor) worker(peerChan <-chan peer.ID) {
	defer pm.wg.Done()
	for {
		select {
		case <-pm.ctx.Done():
			return
		case p, ok := <-peerChan:
			if !ok {
				return
			}
			pm.pingPeer(p)
		}
	}
}

// pingPeer sends a ping to a peer and collects metrics.
func (pm *PerformanceMonitor) pingPeer(p peer.ID) {
	start := time.Now()
	//pm.logger.Debug("Pinging peer", zap.String("peer_id", p.String()))
	ctx, cancel := context.WithTimeout(pm.ctx, pm.metricsTimeout)
	defer cancel()

	stream, err := pm.host.NewStream(ctx, p, PerformanceProtocolID)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "i/o deadline reached") {
			pm.logger.Warn("Failed to create stream for ping",
				zap.String("peer_id", p.String()),
				zap.Error(err),
			)
		} else if errors.Is(err, context.DeadlineExceeded) {
			pm.logger.Warn("Ping timed out",
				zap.String("peer_id", p.String()),
			)
		}
		// On failure, set latency to 0 and success to false
		pm.collector.UpdateResponsiveness(p, 0, false)
		pm.collector.UpdateReliability(p, false)
		return
	}
	defer stream.Close()

	// Send a ping request
	request := []byte("ping")
	_, err = stream.Write(request)
	if err != nil {
		pm.logger.Warn("Failed to send ping",
			zap.String("peer_id", p.String()),
			zap.Error(err),
		)
		// On failure, set latency to 0 and success to false
		pm.collector.UpdateResponsiveness(p, 0, false)
		pm.collector.UpdateReliability(p, false)
		return
	}

	// Await pong response
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			pm.logger.Warn("Ping read timed out",
				zap.String("peer_id", p.String()),
			)
		} else {
			pm.logger.Warn("Failed to read pong",
				zap.String("peer_id", p.String()),
				zap.Error(err),
			)
		}
		// On failure, set latency to 0 and success to false
		pm.collector.UpdateResponsiveness(p, 0, false)
		pm.collector.UpdateReliability(p, false)
		return
	}

	response := string(buf[:n])
	//pm.logger.Debug("Received response from peer", zap.String("peer_id", p.String()), zap.String("response", response))
	if response != "pong" {
		pm.logger.Warn("Invalid pong response",
			zap.String("peer_id", p.String()),
			zap.String("response", response),
		)
		// On failure, set latency to 0 and success to false
		pm.collector.UpdateResponsiveness(p, 0, false)
		pm.collector.UpdateReliability(p, false)
		return
	}

	// Calculate latency
	latency := time.Since(start).Seconds()

	// Record ping result with latency and success
	pm.collector.UpdateResponsiveness(p, latency, true)
	pm.collector.UpdateReliability(p, true)

	//pm.logger.Debug("Received pong from peer", zap.String("peer_id", p.String()), zap.Float64("latency", latency))
}

// cleanupRoutine periodically removes old metrics to prevent memory bloat.
func (pm *PerformanceMonitor) cleanupRoutine() {
	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-pm.cleanupTicker.C:
			pm.collector.RemoveOldMetrics(pm.cleanupThreshold)
			pm.logger.Debug("Cleaned up old peer metrics")
		}
	}
}

// AdjustEmaAlpha dynamically adjusts the EMA smoothing factor.
func (pm *PerformanceMonitor) AdjustEmaAlpha(newAlpha float64) {
	pm.collector.AdjustEmaAlpha(newAlpha)
	pm.logger.Info("Adjusted EMA alpha", zap.Float64("newAlpha", newAlpha))
}

// Stop halts the performance monitoring process gracefully.
func (pm *PerformanceMonitor) Stop() {
	pm.logger.Info("Stopping P2P network performance monitor")
	pm.cancel()
	pm.cleanupTicker.Stop()
	pm.wg.Wait()
	pm.logger.Info("P2P network performance monitor stopped")
}
