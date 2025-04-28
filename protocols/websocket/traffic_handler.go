package websocket

import (
	"github.com/gobwas/ws/wsutil"
	"github.com/panjf2000/gnet/v2"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/transports/tcp"
	"go.uber.org/zap"
)

// TrafficHandler processes WebSocket traffic.
type TrafficHandler struct {
	logger logger.Logger
}

// NewTrafficHandler creates a new instance of TrafficHandler.
func NewTrafficHandler(logger logger.Logger) *TrafficHandler {
	return &TrafficHandler{
		logger: logger,
	}
}

func (h *TrafficHandler) Handle(ctx *tcp.ConnectionContext, conn gnet.Conn) gnet.Action {
	h.logger.Info("Handling incoming connection traffic")

	// Read and accumulate incoming data
	if ctx.ProtocolState == nil {
		ctx.ProtocolState = &WsCodec{
			logger: h.logger,
		}
	}
	codec, ok := ctx.ProtocolState.(*WsCodec)
	if !ok {
		h.logger.Error("Invalid protocol state type")
		return gnet.Close
	}

	// Read available data from the connection
	if codec.ReadBufferBytes(conn) == gnet.Close {
		return gnet.Close
	}

	// Handle WebSocket upgrade if not yet upgraded
	if !codec.upgraded {
		upgraded, action := codec.Upgrade(conn)
		if !upgraded {
			return action
		}
		h.logger.Info("WebSocket upgrade successful", zap.String("remote_addr", conn.RemoteAddr().String()))
	}

	// Decode any complete WebSocket messages
	messages, err := codec.Decode(conn)
	if err != nil {
		h.logger.Error("Failed to decode WebSocket frames", zap.Error(err))
		return gnet.Close
	}

	// Echo the received messages back to the client
	for _, message := range messages {
		h.logger.Info("Echoing message back to client", zap.String("remote_addr", conn.RemoteAddr().String()), zap.ByteString("Payload", message.Payload))
		err = wsutil.WriteServerMessage(conn, message.OpCode, message.Payload)
		if err != nil {
			h.logger.Error("Failed to write WebSocket message", zap.Error(err))
			return gnet.Close
		}
	}

	return gnet.None
}
