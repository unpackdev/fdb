package transports

import (
	"errors"

	"github.com/unpackdev/fdb/pkg/types"

	"github.com/sasha-s/go-deadlock"
)

// Manager is responsible for managing different transport servers
type Manager struct {
	transports map[types.TransportType]Transport // Holds references to different transports
	mu         deadlock.Mutex
}

func NewManager() *Manager {
	return &Manager{
		transports: make(map[types.TransportType]Transport),
	}
}

func (tm *Manager) RegisterTransport(tType types.TransportType, transport Transport) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.transports[tType]; exists {
		return errors.New("transport already registered")
	}

	tm.transports[tType] = transport
	return nil
}

func (tm *Manager) GetTransport(tType types.TransportType) (Transport, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	transport, exists := tm.transports[tType]
	if !exists {
		return nil, errors.New("transport not found")
	}

	return transport, nil
}

func (tm *Manager) GetTransports() map[types.TransportType]Transport {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.transports
}

func (tm *Manager) DeregisterTransport(tType types.TransportType) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.transports[tType]; !exists {
		return errors.New("transport not registered")
	}

	delete(tm.transports, tType)
	return nil
}
