package fdb

import (
	"errors"
	"github.com/unpackdev/fdb/pkg/types"
	"sync"
)

// TransportManager is responsible for managing different transport servers
type TransportManager struct {
	transports map[types.TransportType]Transport // Holds references to different transports
	mu         sync.Mutex
}

func NewTransportManager() *TransportManager {
	return &TransportManager{
		transports: make(map[types.TransportType]Transport),
	}
}

func (tm *TransportManager) RegisterTransport(tType types.TransportType, transport Transport) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.transports[tType]; exists {
		return errors.New("transport already registered")
	}

	tm.transports[tType] = transport
	return nil
}

func (tm *TransportManager) GetTransport(tType types.TransportType) (Transport, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transport, exists := tm.transports[tType]
	if !exists {
		return nil, errors.New("transport not found")
	}

	return transport, nil
}

func (tm *TransportManager) DeregisterTransport(tType types.TransportType) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.transports[tType]; !exists {
		return errors.New("transport not registered")
	}

	delete(tm.transports, tType)
	return nil
}
