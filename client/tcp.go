package client

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/packets"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
)

// bufferSize defines the size of the read buffer
const bufferSize = 128 * 1024 // 128KB buffer

// TCPTransport implements the Transport interface using gnet
type TCPTransport struct {
	address         string
	opts            []gnet.Option
	handlers        map[MessageType]HandlerFunc
	client          *gnet.Client
	conn            gnet.Conn
	mu              sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	logger          logger.Logger
	responseHandler *ResponseHandler
}

// NewTCPTransport creates a new TCPTransport
func NewTCPTransport(address string, logger logger.Logger, opts ...gnet.Option) *TCPTransport {
	// Default response timeout of 30 seconds
	defaultTimeout := 30 * time.Second

	// Create the transport
	transport := &TCPTransport{
		address:         address,
		opts:            opts,
		handlers:        make(map[MessageType]HandlerFunc),
		logger:          logger,
		responseHandler: NewResponseHandler(defaultTimeout),
	}

	// Register default handlers for system messages and error conditions
	RegisterDefaultHandlers(transport)

	return transport
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
	t.mu.Lock()
	defer t.mu.Unlock()

	// Add debug logging
	t.logger.Debug("Registering handler for message type",
		zap.Uint64("type", messageType.Uint64()))

	// Store the handler
	t.handlers[messageType] = handler

	// Verify it was stored correctly
	_, exists := t.handlers[messageType]
	t.logger.Debug("Handler registration status",
		zap.Uint64("type", messageType.Uint64()),
		zap.Bool("stored_successfully", exists))
}

// RegisterResponseChannel registers a channel to receive a response for a specific message type
// and returns the channel that will receive the response
func (t *TCPTransport) RegisterResponseChannel(messageType MessageType) chan []byte {
	return t.responseHandler.RegisterChannel(messageType)
}

// RegisterResponseCallback registers a callback function to handle a response for a specific message type
func (t *TCPTransport) RegisterResponseCallback(messageType MessageType, callback ResponseCallback) {
	t.responseHandler.RegisterCallback(messageType, callback)
}

// WaitForResponse waits for a response on the given channel with the default timeout
func (t *TCPTransport) WaitForResponse(ch chan []byte) ([]byte, error) {
	return t.responseHandler.WaitForResponse(ch)
}

// WaitForResponseWithTimeout waits for a response on the given channel with a custom timeout
func (t *TCPTransport) WaitForResponseWithTimeout(ch chan []byte, timeout time.Duration) ([]byte, error) {
	return t.responseHandler.WaitForResponseWithTimeout(ch, timeout)
}

// UnregisterResponseChannel removes a response channel for a specific message type
func (t *TCPTransport) UnregisterResponseChannel(messageType MessageType) {
	t.responseHandler.UnregisterChannel(messageType)
}

// UnregisterResponseCallback removes a callback for a specific message type
func (t *TCPTransport) UnregisterResponseCallback(messageType MessageType) {
	t.responseHandler.UnregisterCallback(messageType)
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

	// Set the connection context to this handler so message handlers can access it
	c.SetContext(h)

	// Check current registered handlers at connection time
	h.transport.mu.Lock()
	registeredTypes := make([]uint64, 0, len(h.transport.handlers))
	for k := range h.transport.handlers {
		registeredTypes = append(registeredTypes, k.Uint64())
	}
	h.transport.mu.Unlock()

	// Log the current registration state
	h.transport.logger.Debug("Handler registration state at connection time",
		zap.Reflect("registered_handlers", registeredTypes),
		zap.String("connection_id", c.RemoteAddr().String()))

	return nil, gnet.None
}

// OnClose is called when the connection is closed
func (h *tcpEventHandler) OnClose(c gnet.Conn, err error) gnet.Action {
	h.transport.logger.Info("Connection closed", zap.Error(err))
	h.transport.conn = nil
	return gnet.None
}

// pendingLargeResponse holds state for multi-packet response reassembly
type pendingResponse struct {
	expectedSize uint32
	currentSize  uint32
	buffer       []byte
	messageType  MessageType
}

// global state for tracking multi-packet response reassembly
var pendingResp *pendingResponse

// OnTraffic is called when data is received
func (h *tcpEventHandler) OnTraffic(c gnet.Conn) gnet.Action {
	// Create a buffer to hold the complete message
	buf := make([]byte, 0, bufferSize)

	// Read data in a loop until we have the complete message
	for {
		// Read the next chunk of data
		chunk, err := c.Next(-1)
		if err != nil {
			h.transport.logger.Error("Error reading data", zap.Error(err))
			return gnet.Close
		}

		// If no more data, break out of loop
		if len(chunk) == 0 {
			break
		}

		// Append the chunk to our buffer
		buf = append(buf, chunk...)

		// Check if we have a complete message in the buffer
		if isCompleteMessage(buf) {
			break
		}

		// If we received less than what would fill a typical buffer,
		// we've likely received the complete message for now
		if len(chunk) < 4096 {
			break
		}
	}

	data := buf

	// Handle the case where we didn't receive any data
	if len(data) == 0 {
		h.transport.logger.Warn("Received empty data")
		return gnet.None
	}

	// Check if we're continuing a large response reassembly
	if pendingResp != nil {
		// This is a continuation packet of a large response
		h.transport.logger.Debug("Continuing response reassembly",
			zap.Int("chunk_size", len(data)),
			zap.Uint32("current_size", pendingResp.currentSize),
			zap.Uint32("expected_size", pendingResp.expectedSize))

		// Append the new data to our buffer
		pendingResp.buffer = append(pendingResp.buffer, data...)
		pendingResp.currentSize += uint32(len(data))

		// Check if we have the complete message
		if pendingResp.currentSize >= pendingResp.expectedSize {
			// We have the complete message
			h.transport.logger.Debug("Response reassembly complete",
				zap.Uint32("final_size", pendingResp.currentSize),
				zap.Uint64("type", pendingResp.messageType.Uint64()))

			// Use the reassembled data for processing
			messageType := pendingResp.messageType

			// Create a complete response with proper header
			// Format: [status_byte(1)][length(4)][data...]
			completeResponse := make([]byte, 5+len(pendingResp.buffer))
			completeResponse[0] = byte(messageType) // Status byte

			// Set the length field to the actual data length
			binary.BigEndian.PutUint32(completeResponse[1:5], uint32(len(pendingResp.buffer)))

			// Copy the data portion
			copy(completeResponse[5:], pendingResp.buffer)

			// Reset pending response state
			pendingResp = nil

			// Handle the complete message - note we pass the full responseData[1:]
			// because HandleResponse expects everything after the message type
			handled := h.transport.responseHandler.HandleResponse(messageType, completeResponse[1:])
			if !handled {
				h.transport.logger.Warn("No handler for reassembled message type",
					zap.Uint64("type", messageType.Uint64()))
			}

			return gnet.None
		} else {
			// Still waiting for more data
			return gnet.None
		}
	}

	// Get the message type from the first byte
	messageType := MessageType(data[0])

	// Log in a more concise format, showing size, type and preview of first bytes
	h.transport.logger.Debug("Received data",
		zap.Int("size", len(data)),
		zap.Uint64("type", messageType.Uint64()),
		zap.String("first_bytes", fmt.Sprintf("%v", data[:min(10, len(data))])))

	// For Success response messages, we need to check if this might be a large response
	if messageType == MessageType(types.HandlerStatusSuccess.Byte()) && len(data) >= 5 {
		// DB responses include a 4-byte length field after the status byte
		// Format: [status_byte(1)][length(4)][data...]
		expectedLength := binary.BigEndian.Uint32(data[1:5])
		actualDataLength := len(data) - 5 // subtract header bytes

		// If this seems to be a partial response for a large packet
		if expectedLength > uint32(actualDataLength) {
			h.transport.logger.Debug("Detected partial response",
				zap.Uint32("expected_size", expectedLength),
				zap.Int("received_size", actualDataLength))

			// Start response reassembly process
			pendingResp = &pendingResponse{
				expectedSize: expectedLength,
				currentSize:  uint32(actualDataLength),
				buffer:       data[5:], // Store just the data portion, skip the header
				messageType:  messageType,
			}

			// Wait for more data
			return gnet.None
		}
	}

	// Extract the payload (everything after the message type byte)
	payload := data[1:]

	// First try to handle it with the response handler
	handled := h.transport.responseHandler.HandleResponse(messageType, payload)

	// If not handled by response handler, try the traditional handlers
	if !handled {
		// Debug handler lookup
		h.transport.mu.Lock()
		registeredTypes := make([]uint64, 0, len(h.transport.handlers))
		for k := range h.transport.handlers {
			registeredTypes = append(registeredTypes, k.Uint64())
		}
		h.transport.mu.Unlock()

		// Check if our target type is in the map
		hasInvalidActionHandler := false
		for _, t := range registeredTypes {
			if t == uint64(InvalidActionMessageType) {
				hasInvalidActionHandler = true
				break
			}
		}

		// Log the current message type and registered handlers
		h.transport.logger.Debug("Looking up handler",
			zap.Uint64("message_type", messageType.Uint64()),
			zap.Reflect("registered_types", registeredTypes),
			zap.Bool("has_invalid_action_handler", hasInvalidActionHandler),
			zap.Bool("message_is_invalid_action", messageType == InvalidActionMessageType))

		// Special handling for InvalidActionMessageType (0x69)
		// This is a common error response type from the server when handling large payloads
		if messageType == InvalidActionMessageType {
			h.transport.logger.Warn("Received InvalidAction message from server",
				zap.Int("size", len(payload)),
				zap.String("error", string(payload)))

			// Try to decode the error message
			dbResponse, err := packets.DecodeDBResponse(payload)
			if err == nil {
				errorMessage := string(dbResponse.Data)
				h.transport.logger.Error("Server reported error",
					zap.String("message", errorMessage))

				// Convert to a regular error response that waiting handlers can understand
				successMsgType := MessageType(types.HandlerStatusSuccess.Byte())
				h.transport.responseHandler.HandleResponse(successMsgType, payload)
			}

			// Don't fall through to the normal handler lookup
			return gnet.None
		}

		// Normal handler lookup and execution
		handler, exists := h.transport.handlers[messageType]
		if exists {
			h.transport.logger.Debug("Found handler for message type",
				zap.Uint64("type", messageType.Uint64()))
			if err := handler(c, payload); err != nil {
				h.transport.logger.Error("Handler error", zap.Error(err))
			}
		} else {
			h.transport.logger.Warn("No handler for message type",
				zap.Uint64("type", messageType.Uint64()))
		}
	}

	return gnet.None
}

// isCompleteMessage checks if we have a complete valid message in the buffer
// by verifying message structure and length requirements
func isCompleteMessage(data []byte) bool {
	// Need at least 1 byte for message type
	if len(data) < 1 {
		return false
	}

	// Check if message is a success response (which includes a payload length)
	msgType := MessageType(data[0])
	if msgType == MessageType(types.HandlerStatusSuccess.Byte()) {
		// Need at least 5 bytes for header (1 status + 4 length)
		if len(data) < 5 {
			return false
		}

		// Check if actual data length matches expected length from header
		expectedLength := binary.BigEndian.Uint32(data[1:5])
		actualDataLength := len(data) - 5 // subtract header bytes

		// Only complete if we have all the expected data
		return uint32(actualDataLength) >= expectedLength
	}

	// For write/read handler messages which have a specific format
	if msgType == MessageType('W') || msgType == MessageType('R') {
		// These also use a length field - need at least 5 bytes (1 type + 4 length)
		if len(data) < 5 {
			return false
		}

		// Get the expected message length
		expectedLength := binary.BigEndian.Uint32(data[1:5])
		actualDataLength := len(data) - 5

		// Check if we have the complete message
		return uint32(actualDataLength) >= expectedLength
	}

	// For other message types where we can't easily determine completeness
	// by examining headers, use a more conservative approach
	return false
}

// OnTick is called periodically
func (h *tcpEventHandler) OnTick() (time.Duration, gnet.Action) {
	return time.Second, gnet.None
}
