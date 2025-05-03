// pkg/protocols/rpc/server.go
package rpc

import (
	"context"

	json "github.com/goccy/go-json"
	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"
	"go.uber.org/zap"
)

// Server manages RPC handlers and dispatching.
type Server struct {
	handlers         map[HandlerMethodName]HandlerFunc
	mu               deadlock.RWMutex
	logger           logger.Logger
	obs              *observability.Observability
	webSocketHandler *WebSocketHandler
}

// NewServer creates a new Server instance.
func NewServer(logger logger.Logger, obs *observability.Observability) *Server {
	return &Server{
		handlers: make(map[HandlerMethodName]HandlerFunc),
		logger:   logger,
		obs:      obs,
	}
}

func (s *Server) SetWebSocketHandler(handler *WebSocketHandler) {
	s.webSocketHandler = handler
}

// RegisterMethod registers a new RPC method handler.
func (s *Server) RegisterMethod(method HandlerMethodName, handler HandlerFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.handlers[method]; ok {
		return errors.Errorf("rpc method %s already registered", method)
	}
	s.handlers[method] = handler
	s.logger.Debug("Registered RPC method", zap.String("method", method.String()))
	return nil
}

// HandleRequest processes an RPC request and returns a response.
func (s *Server) HandleRequest(ctx context.Context, req Request) Response {
	s.mu.RLock()
	handler, exists := s.handlers[HandlerMethodName(req.Method)]
	s.mu.RUnlock()

	if !exists {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    MethodNotFound,
				Message: "Method not found",
			},
		}
	}

	result, rpcErr := handler(ctx, req.Params)
	if rpcErr != nil {
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// HandleRawRequest handles raw JSON-RPC request bytes and returns the response bytes.
func (s *Server) HandleRawRequest(ctx context.Context, data []byte) ([]byte, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		s.logger.Error("Failed to unmarshal request", zap.Error(err))
		// Return error response
		resp := Response{
			JSONRPC: "2.0",
			ID:      nil, // Unknown due to parse error
			Error: &Error{
				Code:    ParseError,
				Message: "Parse error",
			},
		}
		respBytes, _ := json.Marshal(resp)
		return respBytes, err
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    InvalidRequest,
				Message: "Invalid JSON-RPC version",
			},
		}
		respBytes, _ := json.Marshal(resp)
		return respBytes, nil
	}

	// Include ClientConnection in context if available
	if clientConn, ok := ctx.Value(clientConnKey).(*ClientConnection); ok {
		ctx = context.WithValue(ctx, clientConnKey, clientConn)
	}

	resp := s.HandleRequest(ctx, req)
	respBytes, err := json.Marshal(resp)
	if err != nil {
		s.logger.Error("Failed to marshal response", zap.Error(err))
		// Return internal error response
		resp := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    InternalError,
				Message: "Internal error",
			},
		}
		respBytes, _ := json.Marshal(resp)
		return respBytes, err
	}

	return respBytes, nil
}
