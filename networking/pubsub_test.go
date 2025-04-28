// pkg/networking/pubsub_test.go
package networking

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"log"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
)

func TestPubSubService(t *testing.T) {
	ctx := context.Background()
	logger := log.New(os.Stdout, "PubSubTest: ", log.LstdFlags)

	// Create two hosts for testing purposes.
	h1, err := libp2p.New()
	assert.NoError(t, err, "Failed to create host 1")

	h2, err := libp2p.New()
	assert.NoError(t, err, "Failed to create host 2")

	// Set up PubSub for each host.
	ps1, err := pubsub.NewGossipSub(ctx, h1)
	assert.NoError(t, err, "Failed to create PubSub for host 1")

	ps2, err := pubsub.NewGossipSub(ctx, h2)
	assert.NoError(t, err, "Failed to create PubSub for host 2")

	// Create PubSub services for each host.
	psService1, err := NewPubSubService(ctx, ps1, "TestTopic", logger)
	assert.NoError(t, err, "Failed to create PubSub service for host 1")

	psService2, err := NewPubSubService(ctx, ps2, "TestTopic", logger)
	assert.NoError(t, err, "Failed to create PubSub service for host 2")

	// Connect the hosts.
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	assert.NoError(t, err, "Failed to connect host 1 to host 2")

	// Allow some time for PubSub setup.
	time.Sleep(2 * time.Second)

	// Start reading messages on host 2.
	received := make(chan string, 1)
	go func() {
		for {
			msg, err := psService2.Sub.Next(ctx)
			if err != nil {
				continue
			}
			received <- string(msg.Data)
		}
	}()

	// Publish a message from host 1.
	err = psService1.PublishMessage([]byte("Hello from host 1!"))
	assert.NoError(t, err, "Failed to publish message from host 1")

	// Wait for the message to be received on host 2.
	select {
	case msg := <-received:
		assert.Equal(t, "Hello from host 1!", msg, "Received message should match the sent message")
	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for message")
	}

	// Close the hosts.
	err = h1.Close()
	assert.NoError(t, err, "Failed to close host 1")

	err = h2.Close()
	assert.NoError(t, err, "Failed to close host 2")
}
