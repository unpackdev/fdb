// pkg/networking/notifier.go

package networking

import (
	"context"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/logger"
	"sync/atomic"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

// PeerConnectedHandler defines the callback signature for peer connection events.
type PeerConnectedHandler func(ctx context.Context, peerInfo peer.AddrInfo) error

// PeerDisconnectedHandler defines the callback signature for peer disconnection events.
type PeerDisconnectedHandler func(ctx context.Context, peerID peer.ID) error

// Notifier manages the registration and invocation of peer event handlers.
type Notifier struct {
	mu deadlock.RWMutex

	// Using atomic counters for unique handler IDs
	connectedHandlerCounter    uint64
	disconnectedHandlerCounter uint64

	// Maps to store handlers with their unique IDs
	peerConnectedHandlers    map[uint64]PeerConnectedHandler
	peerDisconnectedHandlers map[uint64]PeerDisconnectedHandler

	logger logger.Logger
}

// NewNotifier initializes and returns a new Notifier instance.
// It accepts a logger to log any internal errors or panics.
func NewNotifier(logger logger.Logger) *Notifier {
	return &Notifier{
		peerConnectedHandlers:    make(map[uint64]PeerConnectedHandler),
		peerDisconnectedHandlers: make(map[uint64]PeerDisconnectedHandler),
		logger:                   logger,
	}
}

// RegisterPeerConnectedHandler registers a new handler for peer connection events.
// It returns a deregistration function to remove the handler when it's no longer needed.
func (n *Notifier) RegisterPeerConnectedHandler(handler PeerConnectedHandler) func() {
	id := atomic.AddUint64(&n.connectedHandlerCounter, 1)

	n.mu.Lock()
	n.peerConnectedHandlers[id] = handler
	n.mu.Unlock()

	return func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		delete(n.peerConnectedHandlers, id)
	}
}

// RegisterPeerDisconnectedHandler registers a new handler for peer disconnection events.
// It returns a deregistration function to remove the handler when it's no longer needed.
func (n *Notifier) RegisterPeerDisconnectedHandler(handler PeerDisconnectedHandler) func() {
	id := atomic.AddUint64(&n.disconnectedHandlerCounter, 1)

	n.mu.Lock()
	n.peerDisconnectedHandlers[id] = handler
	n.mu.Unlock()

	return func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		delete(n.peerDisconnectedHandlers, id)
	}
}

// NotifyPeerConnected invokes all registered peer connected handlers with the given peer information.
func (n *Notifier) NotifyPeerConnected(ctx context.Context, peerInfo peer.AddrInfo) {
	n.mu.RLock()
	handlers := make([]PeerConnectedHandler, 0, len(n.peerConnectedHandlers))
	for _, handler := range n.peerConnectedHandlers {
		handlers = append(handlers, handler)
	}
	n.mu.RUnlock()

	for _, handler := range handlers {
		// Invoke each handler in its own goroutine to prevent blocking.
		go func(h PeerConnectedHandler) {
			defer func() {
				if r := recover(); r != nil {
					n.logger.Error("Recovered from panic in PeerConnectedHandler", zap.Any("recovered", r))
				}
			}()
			h(ctx, peerInfo)
		}(handler)
	}
}

// NotifyPeerDisconnected invokes all registered peer disconnected handlers with the given peer ID.
func (n *Notifier) NotifyPeerDisconnected(ctx context.Context, peerID peer.ID) {
	n.mu.RLock()
	handlers := make([]PeerDisconnectedHandler, 0, len(n.peerDisconnectedHandlers))
	for _, handler := range n.peerDisconnectedHandlers {
		handlers = append(handlers, handler)
	}
	n.mu.RUnlock()

	for _, handler := range handlers {
		// Invoke each handler in its own goroutine to prevent blocking.
		go func(h PeerDisconnectedHandler) {
			defer func() {
				if r := recover(); r != nil {
					n.logger.Error("Recovered from panic in PeerDisconnectedHandler", zap.Any("recovered", r))
				}
			}()
			h(ctx, peerID)
		}(handler)
	}
}

// handlePeerConnected is invoked when a new peer connects to the network.
func (n *Network) handlePeerConnected(net network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	addrs := conn.RemoteMultiaddr()

	// Convert multiaddr to peer.AddrInfo
	addrInfo := peer.AddrInfo{
		ID:    peerID,
		Addrs: []multiaddr.Multiaddr{addrs},
	}

	// Notify all registered peer connected handlers
	n.notifier.NotifyPeerConnected(n.ctx, addrInfo)
}

// handlePeerDisconnected is invoked when a peer disconnects from the network.
func (n *Network) handlePeerDisconnected(net network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()

	// Notify all registered peer disconnected handlers
	n.notifier.NotifyPeerDisconnected(n.ctx, peerID)
}

// RegisterPeerConnectedHandler allows external packages to register a handler for peer connections.
// It returns a registration function.
func (n *Network) RegisterPeerConnectedHandler(handler PeerConnectedHandler) func() {
	return n.notifier.RegisterPeerConnectedHandler(handler)
}

// RegisterPeerDisconnectedHandler allows external packages to register a handler for peer disconnections.
// It returns a deregistration function.
func (n *Network) RegisterPeerDisconnectedHandler(handler PeerDisconnectedHandler) func() {
	return n.notifier.RegisterPeerDisconnectedHandler(handler)
}
