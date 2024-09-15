package fdb

import (
	"errors"
	"github.com/unpackdev/fdb/pkg/types"
	"sync"
)

// TransportManager is responsible for managing different transport servers
type TransportManager struct {
	transports map[types.TransportType]interface{} // Holds references to different transports
	mu         sync.Mutex
}

// NewTransportManager creates a new TransportManager instance
func NewTransportManager() *TransportManager {
	return &TransportManager{
		transports: make(map[types.TransportType]interface{}),
	}
}

// RegisterTransport registers a transport by type
func (tm *TransportManager) RegisterTransport(tType types.TransportType, transport interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if the transport type is already registered
	if _, exists := tm.transports[tType]; exists {
		return errors.New("transport already registered")
	}

	// Register the transport
	tm.transports[tType] = transport
	return nil
}

// GetTransport retrieves a transport by type
func (tm *TransportManager) GetTransport(tType types.TransportType) (interface{}, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transport, exists := tm.transports[tType]
	if !exists {
		return nil, errors.New("transport not found")
	}

	return transport, nil
}

// DeregisterTransport removes a registered transport by type
func (tm *TransportManager) DeregisterTransport(tType types.TransportType) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if the transport exists
	if _, exists := tm.transports[tType]; !exists {
		return errors.New("transport not registered")
	}

	// Deregister the transport
	delete(tm.transports, tType)
	return nil
}
