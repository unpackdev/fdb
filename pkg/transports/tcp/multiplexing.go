// pkg/transports/tcp/multiplexing_handler.go
package tcp

import (
	"bytes"

	"github.com/panjf2000/gnet/v2"
	"github.com/unpackdev/fdb/pkg/logger"
	"go.uber.org/zap"
)

// TrafficHandler is an interface that both HTTPHandler and WebSocketHandler implement.
type TrafficHandler interface {
	Handle(ctx *ConnectionContext, conn gnet.Conn) gnet.Action
	OnClose(ctx *ConnectionContext, conn gnet.Conn)
}

// MultiplexingTrafficHandler multiplexes between different protocol handlers.
type MultiplexingTrafficHandler struct {
	httpHandler TrafficHandler
	wsHandler   TrafficHandler
	logger      logger.Logger
}

func NewMultiplexingTrafficHandler(httpHandler TrafficHandler, wsHandler TrafficHandler, logger logger.Logger) *MultiplexingTrafficHandler {
	return &MultiplexingTrafficHandler{
		httpHandler: httpHandler,
		wsHandler:   wsHandler,
		logger:      logger,
	}
}

func (m *MultiplexingTrafficHandler) Handle(ctx *ConnectionContext, conn gnet.Conn) gnet.Action {
	// If we've already decided on a protocol for this connection, use it.
	if ctx.Protocol == "http" {
		return m.httpHandler.Handle(ctx, conn)
	} else if ctx.Protocol == "websocket" {
		return m.wsHandler.Handle(ctx, conn)
	}

	// Read available data.
	data, err := conn.Peek(conn.InboundBuffered())
	if err != nil {
		m.logger.Error("Failed to read data for protocol detection", zap.Error(err))
		return gnet.Close
	}

	// Check for WebSocket upgrade request.
	if isWebSocketUpgrade(data) {
		m.logger.Debug("Detected WebSocket upgrade request")
		ctx.Protocol = "websocket"
		return m.wsHandler.Handle(ctx, conn)
	}

	// Otherwise, assume HTTP.
	m.logger.Debug("Assuming HTTP protocol")
	ctx.Protocol = "http"
	return m.httpHandler.Handle(ctx, conn)
}

// OnClose delegates the OnClose call to the appropriate protocol handler.
func (m *MultiplexingTrafficHandler) OnClose(ctx *ConnectionContext, conn gnet.Conn) {
	switch ctx.Protocol {
	case "http":
		if handler, ok := m.httpHandler.(interface {
			OnClose(*ConnectionContext, gnet.Conn)
		}); ok {
			handler.OnClose(ctx, conn)
		}
	case "websocket":
		if handler, ok := m.wsHandler.(interface {
			OnClose(*ConnectionContext, gnet.Conn)
		}); ok {
			handler.OnClose(ctx, conn)
		}
	}
}

// Helper function to detect WebSocket upgrade request.
func isWebSocketUpgrade(data []byte) bool {
	return bytes.Contains(data, []byte("Upgrade: websocket"))
}
