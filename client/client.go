package client

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/unpackdev/fdb/pkg/messages"
	"github.com/unpackdev/fdb/pkg/types"
	"sync"
	"time"
)

// Client manages multiple transports and handlers using the config
type Client struct {
	transports map[string]Transport
	ctx        context.Context
	mu         sync.RWMutex
}

// NewClient creates a new Client using the provided config
func NewClient(ctx context.Context, cfg *Config) *Client {
	return &Client{
		ctx:        ctx,
		transports: cfg.Transports,
	}
}

// RegisterTransport adds a transport to the config
func (c *Client) RegisterTransport(name string, transport Transport) error {
	if _, exists := c.transports[name]; exists {
		return fmt.Errorf("transport %s already registered", name)
	}
	c.transports[name] = transport
	return nil
}

// GetTransport retrieves a registered transport by name
func (c *Client) GetTransport(name string) (Transport, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	transport, exists := c.transports[name]
	if !exists {
		return nil, errors.New("transport not found")
	}
	return transport, nil
}

// GetTransportByType retrieves a registered transport by type
func (c *Client) GetTransportByType(name types.TransportType) (Transport, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	transport, exists := c.transports[name.String()]
	if !exists {
		return nil, fmt.Errorf("transport by type not found: %s", name)
	}
	return transport, nil
}

// SendMessage sends a message using the specified transport
func (c *Client) SendMessage(name string, data []byte) error {
	transport, err := c.GetTransport(name)
	if err != nil {
		return err
	}
	return transport.Send(data)
}

// Start starts all transports in the client
func (c *Client) Start(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, transport := range c.transports {
		err := transport.Connect(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close shuts down all transports in the client
func (c *Client) Close() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, transport := range c.transports {
		err := transport.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// SendAndReceiveMessage sends a message and waits for a response with the specified timeout.
func (c *Client) SendAndReceiveMessage(ctx context.Context, transportType string, data []byte, timeout time.Duration) ([]byte, error) {
	// Get the transport
	transport, err := c.GetTransport(transportType)
	if err != nil {
		return nil, err
	}

	// Check if we're dealing with a TCPTransport which has advanced response handling
	tcpTransport, ok := transport.(*TCPTransport)
	if !ok {
		return nil, fmt.Errorf("SendAndReceiveMessage is only supported for TCP transport, got %T", transport)
	}

	// Try to decode the message to extract the UUID
	// If it's not already an encoded message, we'll need to generate a new UUID
	var messageID uuid.UUID

	if decodedMsg, err := messages.Decode(data); err == nil {
		// Data is already an encoded message, extract the ID
		messageID = decodedMsg.ID
	} else {
		// Generate a new UUID for this request
		messageID = uuid.New()
		// Note: we should ideally encode this ID into the data, but for now
		// we'll just use it for correlation. In a complete implementation,
		// you'd want to encode the data with this ID.
	}

	// Register a response channel using the message ID
	responseCh := tcpTransport.RegisterResponseChannel(messageID)

	// Define the maximum size for non-chunked messages
	const maxChunkSize = 1024 * 1024 // 1MB

	// For large payloads, use chunking protocol to ensure reliable transmission
	encodedMsg := data
	if len(data) > maxChunkSize {
		// Prepare the chunked message with a 4-byte length prefix
		// The server expects: [total_size(4 bytes)][payload...]
		lengthPrefix := make([]byte, 4)
		binary.LittleEndian.PutUint32(lengthPrefix, uint32(len(data)))

		// Prepend the length prefix to the message
		chunkedMsg := append(lengthPrefix, data...)

		// Replace the original message with the chunked version
		encodedMsg = chunkedMsg
	}

	// Send the message
	if err := tcpTransport.Send(encodedMsg); err != nil {
		// Make sure to unregister the response channel on error
		tcpTransport.UnregisterResponseChannel(messageID)
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Wait for the response with the provided timeout
	response, err := tcpTransport.WaitForResponseWithTimeout(responseCh, timeout)
	if err != nil {
		return nil, fmt.Errorf("error waiting for response: %w", err)
	}

	return response, nil
}
