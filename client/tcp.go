package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/unpackdev/fdb/pkg/logger"
	"go.uber.org/zap"
	"github.com/google/uuid"
)

// TCPTransport implements the Transport interface using standard net package
type TCPTransport struct {
	address         string
	handlers        map[MessageType]HandlerFunc
	conn            *net.TCPConn
	reader          *bufio.Reader
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	logger          logger.Logger
	responseHandler *ResponseHandler
	readLoop        sync.WaitGroup
	readBuffer      []byte
	stopChan        chan struct{}
}

// NewTCPTransport creates a new TCPTransport
func NewTCPTransport(address string, logger logger.Logger, opts ...interface{}) *TCPTransport {
	transport := &TCPTransport{
		address:    address,
		handlers:   make(map[MessageType]HandlerFunc),
		logger:     logger,
		readBuffer: make([]byte, 64*1024), // 64KB read buffer
		stopChan:   make(chan struct{}),
	}

	// Create new response handler
	transport.responseHandler = NewResponseHandler(30 * time.Second)

	// Register default handlers for system messages and error conditions
	RegisterDefaultHandlers(transport)

	return transport
}

// Connect establishes the TCP connection
func (t *TCPTransport) Connect(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Create TCP address
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.address)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %w", err)
	}

	// Establish connection
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// Configure TCP connection
	err = conn.SetNoDelay(true) // Disable Nagle's algorithm
	if err != nil {
		t.logger.Warn("Failed to set TCP_NODELAY", zap.Error(err))
	}

	err = conn.SetWriteBuffer(256 * 1024) // 256KB write buffer
	if err != nil {
		t.logger.Warn("Failed to set write buffer size", zap.Error(err))
	}

	err = conn.SetReadBuffer(256 * 1024) // 256KB read buffer
	if err != nil {
		t.logger.Warn("Failed to set read buffer size", zap.Error(err))
	}

	t.conn = conn
	t.reader = bufio.NewReader(conn)
	t.logger.Info("Connected to server", zap.String("remote", conn.RemoteAddr().String()))

	return nil
}

// Send sends a message over the TCP connection
func (t *TCPTransport) Send(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return errors.New("no active connection")
	}

	// Log the message being sent
	t.logger.Debug("Sending message to server",
		zap.Int("data_size", len(data)),
		zap.String("first_bytes", fmt.Sprintf("%v", data[:min(10, len(data))])))

	// Write data directly
	_, err := t.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	// After sending, try to read response
	t.logger.Debug("Message sent, attempting to read response")
	if err := t.receiveResponse(); err != nil {
		t.logger.Warn("Failed to receive response", zap.Error(err))
		// Not returning error here as the message was still sent
	}

	return nil
}

// receiveResponse tries to read a response from the server after sending a message
func (t *TCPTransport) receiveResponse() error {
	// Set a read deadline to avoid blocking indefinitely - use a longer timeout
	t.logger.Debug("Setting read deadline for response")
	if err := t.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Try to read data - first check if there's anything to read
	t.logger.Debug("Checking for available data from server")

	// Try to read data
	data := make([]byte, 8192) // Larger buffer for receiving response
	t.logger.Debug("Reading response from server")
	n, err := t.reader.Read(data)

	// Reset the read deadline
	t.conn.SetReadDeadline(time.Time{})

	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// Timeout is expected sometimes, not a critical error
			t.logger.Debug("Read timeout occurred - no response received within timeout")
			return nil
		}
		if err == io.EOF {
			t.logger.Warn("Connection closed by server")
			return fmt.Errorf("connection closed by server")
		}
		t.logger.Error("Error reading response", zap.Error(err))
		return fmt.Errorf("error reading response: %w", err)
	}

	if n > 0 {
		t.logger.Debug("Received response data",
			zap.Int("bytes", n),
			zap.Binary("raw_data", data[:min(20, n)]),
			zap.String("first_bytes", fmt.Sprintf("%v", data[:min(20, n)])))

		// Process the response
		t.processResponseData(data[:n])
	} else {
		t.logger.Warn("Received empty response (0 bytes)")
	}

	return nil
}

// processResponseData handles data received from the server
func (t *TCPTransport) processResponseData(data []byte) {
	// Check if we have a valid message
	if len(data) < 1 {
		t.logger.Warn("Received empty response")
		return
	}

	// Extract message type and payload
	messageType := MessageType(data[0])
	payload := data[1:]

	t.logger.Debug("Processing response",
		zap.Uint64("type", messageType.Uint64()),
		zap.Int("payload_length", len(payload)))

	// Try to handle with response handler first
	if t.responseHandler != nil {
		handled := t.responseHandler.HandleResponse(messageType, payload)
		if handled {
			t.logger.Debug("Response handled by response handler",
				zap.Uint64("type", messageType.Uint64()))
			return
		}
	}

	// Normal handler lookup and execution
	t.mu.Lock()
	handler, exists := t.handlers[messageType]
	t.mu.Unlock()

	if exists {
		t.logger.Debug("Found handler for message type",
			zap.Uint64("type", messageType.Uint64()))
		if err := handler(payload); err != nil {
			t.logger.Error("Handler error", zap.Error(err))
		}
	} else {
		t.logger.Warn("No handler for message type",
			zap.Uint64("type", messageType.Uint64()))
	}
}

// Close closes the TCP connection
func (t *TCPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn != nil {
		close(t.stopChan)
		t.cancel()

		// Close the connection
		err := t.conn.Close()
		t.conn = nil
		t.reader = nil
		if err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	return nil
}

// RegisterHandler registers a handler for a specific message type
func (t *TCPTransport) RegisterHandler(messageType MessageType, handler HandlerFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.handlers[messageType] = handler
	t.logger.Debug("Registered handler for message type",
		zap.Uint64("type", messageType.Uint64()),
		zap.String("hex", fmt.Sprintf("0x%02x", messageType)))

	// Log number of registered handlers
	t.logger.Debug("Current handler count", zap.Int("count", len(t.handlers)))
}

// RegisterResponseChannel registers a channel to receive a response for a specific message ID
// and returns the channel that will receive the response
func (t *TCPTransport) RegisterResponseChannel(messageID uuid.UUID) chan []byte {
	return t.responseHandler.RegisterChannel(messageID)
}

// UnregisterResponseChannel removes a response channel for a specific message ID
func (t *TCPTransport) UnregisterResponseChannel(messageID uuid.UUID) {
	t.responseHandler.UnregisterChannel(messageID)
}

// RegisterResponseCallback registers a callback function to handle a response for a specific message ID
func (t *TCPTransport) RegisterResponseCallback(messageID uuid.UUID, callback ResponseCallback) {
	t.responseHandler.RegisterCallback(messageID, callback)
}

// WaitForResponse waits for a response on the given channel with the default timeout
func (t *TCPTransport) WaitForResponse(ch chan []byte) ([]byte, error) {
	return t.WaitForResponseWithTimeout(ch, 5*time.Second)
}

// WaitForResponseWithTimeout waits for a response on the given channel with a custom timeout
func (t *TCPTransport) WaitForResponseWithTimeout(ch chan []byte, timeout time.Duration) ([]byte, error) {
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout waiting for response")
	}
}

// UnregisterResponseCallback removes a callback for a specific message ID
func (t *TCPTransport) UnregisterResponseCallback(messageID uuid.UUID) {
	t.responseHandler.UnregisterCallback(messageID)
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
