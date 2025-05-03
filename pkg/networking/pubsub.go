package networking

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/core/protocol"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// PubSubService manages PubSub operations for the network.
type PubSubService struct {
	ctx    context.Context
	pubsub *pubsub.PubSub
	topics map[protocol.ID]*pubsub.Topic
}

// NewPubSubService creates a new PubSubService.
func NewPubSubService(ctx context.Context, ps *pubsub.PubSub) *PubSubService {
	return &PubSubService{
		ctx:    ctx,
		pubsub: ps,
		topics: make(map[protocol.ID]*pubsub.Topic),
	}
}

func (ps *PubSubService) PubSub() *pubsub.PubSub {
	return ps.pubsub
}

// JoinTopic joins a PubSub topic and stores it.
func (ps *PubSubService) JoinTopic(topicName protocol.ID, opts ...pubsub.TopicOpt) (*pubsub.Topic, error) {
	topic, err := ps.pubsub.Join(string(topicName), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic %s: %w", topicName, err)
	}
	ps.topics[topicName] = topic
	return topic, nil
}

// PublishMessage publishes a message to a topic.
func (ps *PubSubService) PublishMessage(topicName protocol.ID, message []byte) error {
	topic, ok := ps.topics[topicName]
	if !ok {
		return fmt.Errorf("topic %s not found", topicName)
	}
	return topic.Publish(ps.ctx, message)
}

// Subscribe creates a subscription to a topic.
func (ps *PubSubService) Subscribe(topicName protocol.ID) (*pubsub.Subscription, error) {
	topic, ok := ps.topics[topicName]
	if !ok {
		return nil, fmt.Errorf("topic %s not found", topicName)
	}
	return topic.Subscribe()
}
