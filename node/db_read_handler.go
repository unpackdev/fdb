package node

import (
	"fmt"

	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/packets"
	"github.com/unpackdev/fdb/transports"
	"github.com/unpackdev/fdb/types"

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

		// Create an error response using packets.DBResponse
		errorMsg := "Invalid message format"

		// Create a DBResponse with error status
		dbResp := &packets.DBResponse{
			Status: types.HandlerStatusError,
			Length: uint32(len(errorMsg)),
			Data:   []byte(errorMsg),
		}

		// Encode the response to bytes
		response := dbResp.Encode()
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

		// Create an error response using packets.DBResponse
		errorMsg := "Error reading from database"

		// Create a DBResponse with error status
		dbResp := &packets.DBResponse{
			Status: types.HandlerStatusError,
			Length: uint32(len(errorMsg)),
			Data:   []byte(errorMsg),
		}

		// Encode the response to bytes
		response := dbResp.Encode()
		conn.Send(response)
		return
	}

	if len(value) == 0 {
		rh.logger.Debug(
			"No value found for key",
			zap.Binary("key", key),
		)

		// Create an error response using packets.DBResponse
		errorMsg := "No value found for key"

		// Create a DBResponse with error status
		dbResp := &packets.DBResponse{
			Status: types.HandlerStatusError,
			Length: uint32(len(errorMsg)),
			Data:   []byte(errorMsg),
		}

		// Encode the response to bytes
		response := dbResp.Encode()
		conn.Send(response)
		return
	}

	// Create a success response using packets.DBResponse
	dbResp := &packets.DBResponse{
		Status: types.HandlerStatusSuccess,
		Length: uint32(len(value)),
		Data:   value,
	}

	// Encode the response to bytes
	response := dbResp.Encode()

	fmt.Println("SENDING RESPONSE PREFIX", response[0:10], "with status byte:", response[0], "data length:", dbResp.Length)

	// Send the formatted response back to the client
	conn.Send(response)

	rh.logger.Debug(
		"Sent read response",
		zap.Int("response_size", len(response)),
		zap.Int("value_size", len(value)),
	)
}
