package networking

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"time"
)

// SendToPeer sends data securely to a specific peer using a LibP2P stream.
func (n *Network) SendToPeer(ctx context.Context, peerID peer.ID, protocolID protocol.ID, data []byte) error {
	// Create a new stream to the peer
	stream, err := n.host.NewStream(ctx, peerID, protocolID)
	if err != nil {
		return err
	}
	defer stream.Close()

	// Set a write deadline
	deadline := time.Now().Add(10 * time.Second)
	if err := stream.SetWriteDeadline(deadline); err != nil {
		return err
	}

	// Write data to the stream
	_, err = stream.Write(data)
	if err != nil {
		return err
	}

	// Signal that we're done writing
	if err := stream.CloseWrite(); err != nil {
		return err
	}

	return nil
}
