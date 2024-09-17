package client

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// TCPTransport implements the Transport interface using gnet
type TCPTransport struct {
	address  string
	opts     []gnet.Option
	handlers map[MessageType]HandlerFunc
	client   *gnet.Client
	conn     gnet.Conn
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *zap.Logger
}

// NewTCPTransport creates a new TCPTransport
func NewTCPTransport(address string, logger *zap.Logger, opts ...gnet.Option) *TCPTransport {
	return &TCPTransport{
		address:  address,
		opts:     opts,
		handlers: make(map[MessageType]HandlerFunc),
		logger:   logger,
	}
}

// Connect establishes the TCP connection
func (t *TCPTransport) Connect(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Initialize gnet client
	client, err := gnet.NewClient(&tcpEventHandler{
		transport: t,
	}, t.opts...)
	if err != nil {
		return err
	}
	t.client = client

	// Start the client
	go func() {
		if err := t.client.Start(); err != nil {
			t.logger.Error("Failed to start gnet client", zap.Error(err))
		}
	}()

	// Dial the server
	conn, err := t.client.Dial("tcp", t.address)
	if err != nil {
		return err
	}

	t.conn = conn

	return nil
}

// Send sends a message over the TCP connection
func (t *TCPTransport) Send(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return errors.New("no active connection")
	}

	return t.conn.AsyncWrite(data, nil)
}

// Close closes the TCP connection
func (t *TCPTransport) Close() error {
	if t.client != nil {
		t.cancel()
		return t.client.Stop()
	}
	return nil
}

// RegisterHandler registers a handler for a specific message type
func (t *TCPTransport) RegisterHandler(messageType MessageType, handler HandlerFunc) {
	t.handlers[messageType] = handler
}

// tcpEventHandler implements gnet.EventHandler for the TCPTransport
type tcpEventHandler struct {
	transport *TCPTransport
}

// OnBoot is called when the client starts
func (h *tcpEventHandler) OnBoot(eng gnet.Engine) gnet.Action {
	h.transport.logger.Info("TCP client started")
	return gnet.None
}

// OnShutdown is called when the client is shutting down
func (h *tcpEventHandler) OnShutdown(eng gnet.Engine) {
	h.transport.logger.Info("TCP client shutting down")
}

// OnOpen is called when a new connection is established
func (h *tcpEventHandler) OnOpen(c gnet.Conn) ([]byte, gnet.Action) {
	h.transport.logger.Info("Connected to server", zap.String("remote", c.RemoteAddr().String()))
	h.transport.conn = c // Store the connection
	return nil, gnet.None
}

// OnClose is called when the connection is closed
func (h *tcpEventHandler) OnClose(c gnet.Conn, err error) gnet.Action {
	h.transport.logger.Info("Connection closed", zap.Error(err))
	h.transport.conn = nil
	return gnet.None
}

// OnTraffic is called when data is received
func (h *tcpEventHandler) OnTraffic(c gnet.Conn) gnet.Action {
	data, err := c.Next(-1)
	if err != nil {
		h.transport.logger.Error("Error reading data", zap.Error(err))
		return gnet.Close
	}

	if len(data) < 1 {
		h.transport.logger.Warn("Received empty data")
		return gnet.None
	}

	messageType := MessageType(data[0])
	handler, exists := h.transport.handlers[messageType]
	if exists {
		if err := handler(c, data[1:]); err != nil {
			h.transport.logger.Error("Handler error", zap.Error(err))
		}
	} else {
		h.transport.logger.Warn("No handler for message type", zap.Uint64("type", messageType.Uint64()))
	}

	return gnet.None
}

// OnTick is called periodically
func (h *tcpEventHandler) OnTick() (time.Duration, gnet.Action) {
	// Implement if needed
	return time.Second, gnet.None
}
