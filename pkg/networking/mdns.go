package networking

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sasha-s/go-deadlock"
	"go.uber.org/zap"
)

// PeerConnectedCallback type for successful peer connections.
type PeerConnectedCallback func(context.Context, peer.AddrInfo)

// PeerConnectionFailedCallback type for failed peer connections.
type PeerConnectionFailedCallback func(context.Context, peer.AddrInfo, error)

// PeerDisconnectedCallback type for peer disconnections.
type PeerDisconnectedCallback func(context.Context, peer.ID)

// RetryConfig holds the configuration for retry attempts.
type RetryConfig struct {
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
}

// DefaultRetryConfig provides a default configuration for retries.
var DefaultRetryConfig = RetryConfig{
	MaxRetries:      5,
	InitialInterval: 500 * time.Millisecond,
	MaxInterval:     10 * time.Second,
	Multiplier:      2,
}

// MdnsNotifier implements the mdns.Notifee interface to handle peer discovery via mDNS.
// It supports multiple callbacks for each event type.
type MdnsNotifier struct {
	n *Network

	// Retry configuration
	retryConfig RetryConfig

	// Slices to hold multiple callbacks for each event type.
	peerConnectedCallbacks        []PeerConnectedCallback
	peerConnectionFailedCallbacks []PeerConnectionFailedCallback
	peerDisconnectedCallbacks     []PeerDisconnectedCallback

	// Mutex to ensure thread-safe access to callback slices.
	mutex deadlock.RWMutex
}

// NewMdnsNotifier creates a new MdnsNotifier instance with default retry configuration.
func NewMdnsNotifier(network *Network) *MdnsNotifier {
	return &MdnsNotifier{
		n:           network,
		retryConfig: DefaultRetryConfig,
	}
}

// SetRetryConfig allows setting a custom retry configuration.
func (mdns *MdnsNotifier) SetRetryConfig(config RetryConfig) {
	mdns.retryConfig = config
}

// SetPeerConnectedCallback registers a new callback to be invoked upon successful peer connection.
func (mdns *MdnsNotifier) SetPeerConnectedCallback(callback PeerConnectedCallback) {
	mdns.mutex.Lock()
	defer mdns.mutex.Unlock()
	mdns.peerConnectedCallbacks = append(mdns.peerConnectedCallbacks, callback)
}

// SetPeerConnectionFailedCallback registers a new callback to be invoked upon failed peer connection.
func (mdns *MdnsNotifier) SetPeerConnectionFailedCallback(callback PeerConnectionFailedCallback) {
	mdns.mutex.Lock()
	defer mdns.mutex.Unlock()
	mdns.peerConnectionFailedCallbacks = append(mdns.peerConnectionFailedCallbacks, callback)
}

// SetPeerDisconnectedCallback registers a new callback to be invoked upon peer disconnection.
func (mdns *MdnsNotifier) SetPeerDisconnectedCallback(callback PeerDisconnectedCallback) {
	mdns.mutex.Lock()
	defer mdns.mutex.Unlock()
	mdns.peerDisconnectedCallbacks = append(mdns.peerDisconnectedCallbacks, callback)
}

// HandlePeerFound is called when a new peer is discovered via mDNS.
// It attempts to connect to the discovered peer and invokes the appropriate callbacks.
func (mdns *MdnsNotifier) HandlePeerFound(pi peer.AddrInfo) {
	startTime := time.Now()
	
	// Record metrics for MDNS peer discovery
	if mdns.n.discovery != nil && mdns.n.discovery.Metrics != nil {
		mdns.n.discovery.Metrics.RecordMdnsPeerDiscovered(mdns.n.ctx, 1)
		mdns.n.discovery.Metrics.RecordPeersDiscovered(mdns.n.ctx, 1)
		mdns.n.discovery.Metrics.RecordDiscoveryLatency(mdns.n.ctx, time.Since(startTime))
	}
	
	mdns.n.Logger.Info("mDNS discovery found peer",
		zap.String("peer_id", pi.ID.String()),
		zap.Strings("addresses", addrStrings(pi.Addrs)),
	)
	if len(pi.Addrs) == 0 {
		mdns.n.Logger.Warn("Discovered peer has no addresses", zap.String("peer_id", pi.ID.String()))
		return
	}

	// Define the operation to attempt connection
	operation := func() error {
		err := mdns.n.ConnectPeerInfo(pi)
		if err != nil {
			mdns.n.Logger.Debug("Failed to connect to discovered peer via mDNS",
				zap.String("peer_id", pi.ID.String()), zap.Error(err))
			return err
		}
		mdns.n.Logger.Info("Successfully connected to discovered peer via mDNS",
			zap.String("peer_id", pi.ID.String()))
		return nil
	}

	// Configure exponential backoff
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = mdns.retryConfig.InitialInterval
	backoffConfig.MaxInterval = mdns.retryConfig.MaxInterval
	backoffConfig.Multiplier = mdns.retryConfig.Multiplier
	backoffConfig.MaxElapsedTime = 0 // We'll handle max retries ourselves

	// Create a limited retry strategy
	retryStrategy := backoff.WithContext(backoff.WithMaxRetries(backoffConfig, uint64(mdns.retryConfig.MaxRetries)), mdns.n.ctx)

	// Attempt to connect with retries
	err := backoff.Retry(operation, retryStrategy)
	if err != nil {
		// After exhausting retries, invoke failure callbacks
		mdns.invokeConnectionFailedCallbacks(mdns.n.ctx, pi, err)
	} else {
		// On successful connection, invoke success callbacks
		mdns.invokeConnectionSuccessCallbacks(mdns.n.ctx, pi)
	}
}

// HandlePeerLost is called when a previously discovered peer is no longer reachable via mDNS.
// It invokes all registered disconnection callbacks.
func (mdns *MdnsNotifier) HandlePeerLost(pi peer.AddrInfo) {
	// Record metrics for lost MDNS peers
	if mdns.n.discovery != nil && mdns.n.discovery.Metrics != nil {
		mdns.n.discovery.Metrics.RecordMdnsPeerLost(mdns.n.ctx, 1)
		// Also update active peers count - decrease by 1 since we lost a peer
		mdns.n.discovery.Metrics.RecordActivePeers(mdns.n.ctx, -1)
	}

	mdns.n.Logger.Info("mDNS discovery lost peer",
		zap.String("peer_id", pi.ID.String()),
		zap.Strings("addresses", addrStrings(pi.Addrs)),
	)
	// Invoke all disconnection callbacks if any are set
	mdns.mutex.RLock()
	defer mdns.mutex.RUnlock()
	for _, callback := range mdns.peerDisconnectedCallbacks {
		// Invoke callbacks in separate goroutines to prevent blocking
		go callback(mdns.n.ctx, pi.ID)
	}
}

// invokeConnectionFailedCallbacks safely invokes all registered failure callbacks.
func (mdns *MdnsNotifier) invokeConnectionFailedCallbacks(ctx context.Context, pi peer.AddrInfo, err error) {
	// Record metrics for failed MDNS connections
	if mdns.n.discovery != nil && mdns.n.discovery.Metrics != nil {
		mdns.n.discovery.Metrics.RecordMdnsConnectionFailed(ctx, 1)
	}

	mdns.mutex.RLock()
	defer mdns.mutex.RUnlock()
	for _, callback := range mdns.peerConnectionFailedCallbacks {
		// Invoke callbacks in separate goroutines to prevent blocking
		go callback(ctx, pi, err)
	}
}

// invokeConnectionSuccessCallbacks safely invokes all registered success callbacks.
func (mdns *MdnsNotifier) invokeConnectionSuccessCallbacks(ctx context.Context, pi peer.AddrInfo) {
	// Record metrics for successful MDNS connections
	if mdns.n.discovery != nil && mdns.n.discovery.Metrics != nil {
		mdns.n.discovery.Metrics.RecordMdnsConnectionSuccess(ctx, 1)
	}

	mdns.mutex.RLock()
	defer mdns.mutex.RUnlock()
	for _, callback := range mdns.peerConnectedCallbacks {
		// Invoke callbacks in separate goroutines to prevent blocking
		go callback(ctx, pi)
	}
}
