// pkg/protocols/rpc/websocket_handler.go
package rpc

import (
	"bytes"
	"context"
	"errors"
	"github.com/gobwas/ws/wsutil"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/protocols/websocket"
	"github.com/unpackdev/fdb/transports/tcp"
	"sync"

	"github.com/gobwas/ws"
	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// Use a custom type for context key
type contextKey string

const clientConnKey contextKey = "clientConn"

type WebSocketHandler struct {
	server            *Server
	logger            logger.Logger
	clientConnections sync.Map // Maps gnet.Conn to *ClientConnection
}

var (
	writeBufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
)

func NewWebSocketHandler(server *Server, logger logger.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		server: server,
		logger: logger,
	}
}

func (h *WebSocketHandler) Handle(ctx *tcp.ConnectionContext, conn gnet.Conn) gnet.Action {
	// Read and accumulate incoming data
	if ctx.ProtocolState == nil {
		ctx.ProtocolState = websocket.NewWsCodec(h.logger)
	}
	codec, ok := ctx.ProtocolState.(*websocket.WsCodec)
	if !ok {
		h.logger.Error("Invalid protocol state type")
		return gnet.Close
	}

	// Read available data from the connection
	if codec.ReadBufferBytes(conn) == gnet.Close {
		return gnet.Close
	}

	// Handle WebSocket upgrade if not yet upgraded
	if !codec.Upgraded() {
		upgraded, action := codec.Upgrade(conn)
		if !upgraded {
			return action
		}
		h.logger.Info("WebSocket upgrade successful", zap.String("remote_addr", conn.RemoteAddr().String()))
	}

	// Get or create the ClientConnection
	clientConn, _ := h.getClientConnection(conn, ctx)

	// Use the connection's context
	rpcContext := clientConn.Ctx

	// Decode any complete WebSocket messages
	messages, err := codec.Decode(conn)
	if err != nil {
		var closedErr wsutil.ClosedError
		if errors.As(err, &closedErr) {
			h.logger.Debug("WebSocket connection closed", zap.String("remote_addr", conn.RemoteAddr().String()), zap.Error(err))
			return gnet.Close
		}
		h.logger.Error("Failed to decode WebSocket frames", zap.Error(err))
		return gnet.Close
	}

	// Process the received messages
	for _, message := range messages {
		h.logger.Debug("Processing WebSocket message", zap.String("remote_addr", conn.RemoteAddr().String()))

		switch message.OpCode {
		case ws.OpText, ws.OpBinary:
			// Pass message.Payload to the RPC server
			respBytes, err := h.server.HandleRawRequest(rpcContext, message.Payload)
			if err != nil {
				h.logger.Error("RPC processing failed", zap.Error(err))
				// Optionally, send an error message back
				errMsg := []byte("Internal Server Error")
				h.asyncWriteWebSocketMessage(conn, ws.OpText, errMsg)
				return gnet.Close
			}
			// Send the response back over WebSocket asynchronously
			h.asyncWriteWebSocketMessage(conn, message.OpCode, respBytes)
		case ws.OpClose:
			// Handle close frame
			h.logger.Debug("Received close frame", zap.String("remote_addr", conn.RemoteAddr().String()))
			// Send close frame back
			h.asyncWriteWebSocketMessage(conn, ws.OpClose, nil)
			return gnet.Close
		default:
			// Handle other opcodes if necessary
		}
	}

	return gnet.None
}

// Helper method to write WebSocket messages asynchronously
func (h *WebSocketHandler) asyncWriteWebSocketMessage(conn gnet.Conn, opcode ws.OpCode, payload []byte) {
	buf := writeBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer writeBufferPool.Put(buf)

	frame := ws.NewFrame(opcode, true, payload)
	if err := ws.WriteFrame(buf, frame); err != nil {
		h.logger.Error("Failed to write WebSocket frame to buffer", zap.Error(err))
		return
	}

	conn.AsyncWrite(buf.Bytes(), nil)
}

// Helper method to get or create a ClientConnection
func (h *WebSocketHandler) getClientConnection(conn gnet.Conn, ctx *tcp.ConnectionContext) (*ClientConnection, bool) {
	value, exists := h.clientConnections.Load(conn)
	if exists {
		return value.(*ClientConnection), true
	}

	// Use the connection's context
	clientCtx, cancel := context.WithCancel(ctx.Ctx)

	clientConn := &ClientConnection{
		Conn:          conn,
		Subscriptions: make(map[string]*Subscription),
		Cancel:        cancel,
	}

	// Add clientConn to context
	clientCtx = context.WithValue(clientCtx, clientConnKey, clientConn)
	clientConn.Ctx = clientCtx

	h.clientConnections.Store(conn, clientConn)

	return clientConn, false
}

// OnClose cleans up the client connection when the connection is closed.
func (h *WebSocketHandler) OnClose(ctx *tcp.ConnectionContext, conn gnet.Conn) {
	if value, exists := h.clientConnections.Load(conn); exists {
		clientConn := value.(*ClientConnection)
		clientConn.Cancel() // Cancel the context
		h.clientConnections.Delete(conn)
	}
}
