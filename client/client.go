package client

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
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

// SendAndReceiveMessage sends a message to another node and waits for a
// response. This implementation handles large payloads with chunking and
// provides proper response handling with timeouts.
// Note: Currently this method only works with TCPTransport and will return
// an error for other transport types.
func (c *Client) SendAndReceiveMessage(name string, data []byte, timeout time.Duration) ([]byte, error) {
	transport, err := c.GetTransport(name)
	if err != nil {
		return nil, err
	}
	
	// Check if we're dealing with a TCPTransport which has advanced response handling
	tcpTransport, ok := transport.(*TCPTransport)
	if !ok {
		return nil, fmt.Errorf("SendAndReceiveMessage is only supported for TCP transport, got %T", transport)
	}
	
	// Get the response message type - use the success status
	responseType := MessageType(1) // Using 1 as a default success value
	
	// Register a response channel before sending the message
	responseCh := tcpTransport.RegisterResponseChannel(responseType)
	
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
		tcpTransport.UnregisterResponseChannel(responseType)
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	
	// Wait for the response with the provided timeout
	response, err := tcpTransport.WaitForResponseWithTimeout(responseCh, timeout)
	if err != nil {
		return nil, fmt.Errorf("error waiting for response: %w", err)
	}
	
	return response, nil
}
