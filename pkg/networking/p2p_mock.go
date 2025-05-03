// pkg/networking/mock_p2p_network.go

package networking

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/pkg/logger"

	"github.com/sasha-s/go-deadlock"
	"go.uber.org/zap"
)

// MockSubscription is a mock implementation of the Subscription interface.
type MockSubscription struct {
	mu       deadlock.Mutex
	messages []*Message
	closed   bool
}

// NewMockSubscription initializes and returns a new MockSubscription.
func NewMockSubscription() *MockSubscription {
	return &MockSubscription{
		messages: make([]*Message, 0),
	}
}

// EnqueueMessage allows tests to enqueue messages that will be returned by Next().
func (ms *MockSubscription) EnqueueMessage(message *Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.messages = append(ms.messages, message)
}

// Next returns the next message in the subscription.
// If no messages are available, it returns nil without blocking.
func (ms *MockSubscription) Next(ctx context.Context) (*Message, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.closed {
		return nil, context.Canceled
	}

	if len(ms.messages) == 0 {
		return nil, nil
	}

	msg := ms.messages[0]
	ms.messages = ms.messages[1:]

	return msg, nil
}

// Cancel marks the subscription as closed.
func (ms *MockSubscription) Cancel() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.closed = true
}

// MockP2PNetwork is a mock implementation of the P2PNetworkInterface.
type MockP2PNetwork struct {
	mu            deadlock.Mutex
	broadcasted   []*Message
	subscriptions map[string]*MockSubscription
	hostID        peer.ID
	broadcastCh   chan struct{}
	logger        logger.Logger
}

// NewMockP2PNetwork initializes a new MockP2PNetwork.
func NewMockP2PNetwork(hostID peer.ID, logger logger.Logger) *MockP2PNetwork {
	return &MockP2PNetwork{
		broadcasted:   make([]*Message, 0),
		subscriptions: make(map[string]*MockSubscription),
		hostID:        hostID,
		broadcastCh:   make(chan struct{}, 100),
		logger:        logger,
	}
}

// BroadcastMessage records the broadcasted message and enqueues it to all subscriptions.
func (m *MockP2PNetwork) BroadcastMessage(message []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := &Message{Data: message}
	m.broadcasted = append(m.broadcasted, msg)

	// Enqueue the message into all subscriptions
	for _, sub := range m.subscriptions {
		sub.EnqueueMessage(msg)
	}

	m.logger.Debug("Broadcasted message", zap.Any("message", msg.Data))

	// Notify that a message was broadcasted
	select {
	case m.broadcastCh <- struct{}{}:
	default:
		// If the channel is full, avoid blocking
	}

	return nil
}

// PubSubSubscribe returns a mock subscription for the given topic.
func (m *MockP2PNetwork) PubSubSubscribe(topic string) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mockSub, exists := m.subscriptions[topic]
	if !exists {
		mockSub = NewMockSubscription()
		m.subscriptions[topic] = mockSub
	}
	return mockSub, nil
}

// GetBroadcastedMessages returns all messages that have been broadcasted.
func (m *MockP2PNetwork) GetBroadcastedMessages() []*Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.broadcasted
}

// BroadcastCh returns the broadcast channel.
func (m *MockP2PNetwork) BroadcastCh() <-chan struct{} {
	return m.broadcastCh
}

// GetBroadcastedCount returns the number of broadcasted messages.
func (m *MockP2PNetwork) GetBroadcastedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.broadcasted)
}

// HostID returns the mock host's peer ID.
func (m *MockP2PNetwork) HostID() peer.ID {
	return m.hostID
}
