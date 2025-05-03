package networking

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Subscription defines the methods for receiving messages from a subscription.
type Subscription interface {
	Next(ctx context.Context) (*Message, error)
	Cancel()
}

// P2PNetworkInterface defines the methods required by the ConsensusProtocol.
type P2PNetworkInterface interface {
	BroadcastMessage(message []byte) error
	PubSubSubscribe(topic string) (Subscription, error)
	HostID() peer.ID
}
