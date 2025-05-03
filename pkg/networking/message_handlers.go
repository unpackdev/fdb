// networking/message_handlers.go

package networking

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/packets"

	"github.com/sasha-s/go-deadlock"
	"go.uber.org/zap"
)

// PacketHandlerFunc defines the signature for packet handler functions.
type PacketHandlerFunc func(ctx context.Context, msg *packets.NetworkPacket, sender peer.ID) error

// PacketHandlerRegistry manages registration and invocation of packet handlers based on packet types.
type PacketHandlerRegistry struct {
	handlers map[packets.PacketType]PacketHandlerFunc
	mu       deadlock.RWMutex
	logger   logger.Logger
}

// NewMessageHandlerRegistry initializes a new MessageHandlerRegistry.
func NewMessageHandlerRegistry(logger logger.Logger) *PacketHandlerRegistry {
	return &PacketHandlerRegistry{
		handlers: make(map[packets.PacketType]PacketHandlerFunc),
		logger:   logger,
	}
}

// RegisterHandler registers a handler for a specific message type.
func (mhr *PacketHandlerRegistry) RegisterHandler(packetType packets.PacketType, handler PacketHandlerFunc) {
	mhr.mu.Lock()
	defer mhr.mu.Unlock()
	mhr.handlers[packetType] = handler
	mhr.logger.Info("Registered message handler", zap.String("packet_type", packetType.String()))
}

// HandlePacket invokes the appropriate handler based on the message type.
func (mhr *PacketHandlerRegistry) HandlePacket(ctx context.Context, msg *packets.NetworkPacket, sender peer.ID) error {
	mhr.mu.RLock()
	handler, exists := mhr.handlers[msg.Type]
	mhr.mu.RUnlock()

	if !exists {
		mhr.logger.Warn("No handler registered for packet type", zap.String("type", msg.Type.String()))
		return fmt.Errorf("no handler for message type: %s", msg.Type.String())
	}

	return handler(ctx, msg, sender)
}
