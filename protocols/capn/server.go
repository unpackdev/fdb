// pkg/protocols/capn/server.go
package capn

import (
	"capnproto.org/go/capnp/v3"
	"context"
	"fmt"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/protocols/capn/schema"

	"sync"
)

// Server manages Cap'n Proto message handling and dispatching
type Server struct {
	db     db.Provider
	writer *db.BatchWriter
	mu     sync.RWMutex
	logger logger.Logger
}

// NewServer creates a new Cap'n Proto server instance
func NewServer(db db.Provider, writer *db.BatchWriter, logger logger.Logger) *Server {
	return &Server{
		db:     db,
		writer: writer,
		logger: logger,
	}
}

// Handle processes a Cap'n Proto message and returns a response
// This method is designed to work with transports like TCP or WebSocket
func (s *Server) Handle(conn Connection, frame []byte) {
	// Handle as Cap'n Proto message
	response, err := s.HandleMessage(context.Background(), frame)
	if err != nil {
		s.logger.Error("Failed to handle Cap'n Proto message", "error", err)
		conn.Send([]byte{0x01}) // Error code
		return
	}

	// Send response
	conn.Send(response)
}

// Connection defines the interface for sending data back to a client
type Connection interface {
	Send(data []byte) error
}

// HandleMessage processes a Cap'n Proto message and returns a response
func (s *Server) HandleMessage(ctx context.Context, data []byte) ([]byte, error) {
	// Parse as Cap'n Proto message
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Read the request
	request, err := schema.ReadRootRequest(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to read request: %w", err)
	}

	// Process based on request type
	switch request.Which() {
	case schema.Request_Which_get:
		// Handle get request
		get := request.Get()
		keyBytes, err := get.Key()
		if err != nil {
			return CreateErrorResponse(fmt.Sprintf("failed to get key: %v", err))
		}

		// Read from database
		value, err := s.db.Get(keyBytes)
		if err != nil {
			return CreateErrorResponse(fmt.Sprintf("database error: %v", err))
		}

		if len(value) == 0 {
			return CreateGetResponse(nil, false)
		}

		return CreateGetResponse(value, true)

	case schema.Request_Which_set:
		// Handle set request
		set := request.Set()
		keyBytes, err := set.Key()
		if err != nil {
			return CreateErrorResponse(fmt.Sprintf("failed to get key: %v", err))
		}

		valueBytes, err := set.Value()
		if err != nil {
			return CreateErrorResponse(fmt.Sprintf("failed to get value: %v", err))
		}

		// Convert to [32]byte key if needed
		if len(keyBytes) <= 32 {
			var key [32]byte
			copy(key[:], keyBytes)

			// Buffer the write
			if s.writer != nil {
				s.writer.BufferWrite(key, valueBytes)
			} else {
				// Direct write if no batch writer
				if err := s.db.Set(keyBytes, valueBytes); err != nil {
					return CreateErrorResponse(fmt.Sprintf("database error: %v", err))
				}
			}
		} else {
			// Direct write for larger keys
			if err := s.db.Set(keyBytes, valueBytes); err != nil {
				return CreateErrorResponse(fmt.Sprintf("database error: %v", err))
			}
		}

		return CreateSetResponse(true)

	case schema.Request_Which_delete:
		// Handle delete request
		del := request.Delete()
		keyBytes, err := del.Key()
		if err != nil {
			return CreateErrorResponse(fmt.Sprintf("failed to get key: %v", err))
		}

		// Delete the key
		if err := s.db.Delete(keyBytes); err != nil {
			return CreateErrorResponse(fmt.Sprintf("database error: %v", err))
		}

		return CreateDeleteResponse(true)

	default:
		return CreateErrorResponse("unknown request type")
	}
}
