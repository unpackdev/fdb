package node

import (
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/transports"

	"go.uber.org/zap"
)

// DbReadHandler struct with MDBX database passed in
type DbReadHandler struct {
	db     db.Provider // MDBX database instance
	logger logger.Logger
}

// NewDbReadHandler creates a new DbReadHandler with an MDBX database
func NewDbReadHandler(db db.Provider, logger logger.Logger, obs *observability.Observability, distributor *P2PDistributor) *DbReadHandler {
	return &DbReadHandler{
		db:     db,
		logger: logger,
	}
}

// Handle processes the incoming message using the DbReadHandler
func (rh *DbReadHandler) Handle(conn transports.Connection, frame []byte) {
	rh.logger.Debug(
		"Received read packet",
		zap.String("remote_addr", conn.RemoteAddr()),
		zap.Int("packet_size", len(frame)),
	)

	if len(frame) < 33 { // 1 byte action + 32-byte key
		rh.logger.Debug(
			"Invalid message length",
			zap.Int("length", len(frame)),
			zap.Int("expected", 33),
		)

		// Error format: [status byte = 1][error message]
		errorMsg := "Invalid message format"
		response := make([]byte, 1+len(errorMsg))
		response[0] = 1 // Error status
		copy(response[1:], errorMsg)
		conn.Send(response)
		return
	}

	// Extract the key (32 bytes starting from the second byte)
	key := frame[1:33]

	rh.logger.Debug(
		"Processing read request",
		zap.Binary("key_prefix", key[:8]),
	) // Log first 8 bytes of key for identification

	value, err := rh.db.Get(key)
	if err != nil {
		rh.logger.Error(
			"Error reading from database",
			zap.Error(err),
			zap.Binary("key", key),
		)

		// Error format: [status byte = 1][error message]
		errorMsg := "Error reading from database"
		response := make([]byte, 1+len(errorMsg))
		response[0] = 1 // Error status
		copy(response[1:], errorMsg)
		conn.Send(response)
		return
	}

	if len(value) == 0 {
		rh.logger.Debug(
			"No value found for key",
			zap.Binary("key", key),
		)

		// Error format: [status byte = 1][error message]
		errorMsg := "No value found for key"
		response := make([]byte, 1+len(errorMsg))
		response[0] = 1 // Error status
		copy(response[1:], errorMsg)
		conn.Send(response)
		return
	}

	// Create a response buffer with:  [status byte][value...]
	response := make([]byte, 1+len(value))
	// Set status byte (0 = success)
	response[0] = 0
	// Copy the value after the status byte
	copy(response[1:], value)

	// Send the formatted response back to the client
	conn.Send(response)

	rh.logger.Debug(
		"Sent read response",
		zap.Int("response_size", len(response)),
		zap.Int("value_size", len(value)),
	)
}
