package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
