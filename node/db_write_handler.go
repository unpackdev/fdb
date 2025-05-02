package node

import (
	"encoding/binary"
	"fmt"

	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/packets"
	"github.com/unpackdev/fdb/transports"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
)

// DbWriteHandler struct with MDBX database passed in
type DbWriteHandler struct {
	db          db.Provider     // MDBX database instance
	writer      *db.BatchWriter // Batch writer instance
	logger      logger.Logger
	obs         *observability.Observability
	distributor *P2PDistributor // P2P record distributor for propagating writes
}

// NewDbWriteHandler creates a new DbWriteHandler with an MDBX database
func NewDbWriteHandler(db db.Provider, batchWriter *db.BatchWriter, logger logger.Logger, obs *observability.Observability, distributor *P2PDistributor) *DbWriteHandler {
	return &DbWriteHandler{
		db:          db,
		writer:      batchWriter,
		logger:      logger,
		obs:         obs,
		distributor: distributor,
	}
}

// ForceFlush forces the BatchWriter to flush any pending writes.
// This is particularly useful for testing scenarios where immediate persistence is needed.
func (wh *DbWriteHandler) ForceFlush() {
	if wh.writer != nil {
		wh.writer.Flush()
	}
}

// Handle processes the incoming message using the TCPWriteHandler
func (wh *DbWriteHandler) Handle(conn transports.Connection, frame []byte) {
	fmt.Println("DID I REACH WRITE DB HANLDER? FRAME LENGTH:", len(frame))
	// Debug the raw incoming frame
	fmt.Printf("WRITE HANDLER RAW FRAME (first 20 bytes): %v\n", frame[:min(20, len(frame))])

	// Check if first byte is our special marker 0xF0
	offset := 0
	if len(frame) > 0 && frame[0] == 0xF0 {
		offset = 1 // Skip the marker byte
		fmt.Println("DETECTED MARKER BYTE 0xF0, offset set to", offset)
	}

	// Adjust minimum length check based on whether we have a marker
	minLen := 34 // 1 byte for action, 32 bytes for key, and at least 1 byte for value
	if offset > 0 {
		minLen = 35 // Extra byte for the marker
	}

	if len(frame) < minLen {
		wh.logger.Error("Invalid message length",
			zap.Int("length", len(frame)),
			zap.Int("expected", minLen))

		// Create an error response using packets.DBResponse
		errorMsg := "Invalid message length"
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

	// Debug action byte
	actionByte := frame[offset]
	fmt.Printf("ACTION BYTE: 0x%02x\n", actionByte)

	// Create a [32]byte key from the frame without using the pool
	var key [32]byte
	copy(key[:], frame[offset+1:offset+33]) // Copy directly from frame, accounting for offset

	// Debug the key being extracted
	fmt.Printf("KEY (first 8 bytes): %v\n", key[:8])

	// For the message protocol format: [1-byte action][32-byte key][4-byte length][actual data]
	// We need to skip 4 bytes after the key to get to the actual data
	valueStart := offset + 33 + 4 // Skip action byte (1) + key (32) + length field (4)

	// Make sure the frame is large enough to include at least some data
	if len(frame) <= valueStart {
		wh.logger.Error("Frame too small to contain data",
			zap.Int("frame_size", len(frame)),
			zap.Int("min_needed", valueStart+1))

		// Return an error response
		errorMsg := "Invalid message format"
		dbResp := &packets.DBResponse{
			Status: types.HandlerStatusError,
			Length: uint32(len(errorMsg)),
			Data:   []byte(errorMsg),
		}
		conn.Send(dbResp.Encode())
		return
	}

	// Extract only the actual data, skipping the length field
	value := frame[valueStart:]

	fmt.Printf("ACTUAL VALUE (first %d bytes): %v\n", min(10, len(value)), value[:min(10, len(value))])

	// Add detailed logging for debugging large payload issues
	wh.logger.Debug("Received write request",
		zap.Int("frame_size", len(frame)),
		zap.Int("value_size", len(value)),
		zap.Int("offset", offset),
		zap.Binary("key_prefix", key[:4]),
		zap.Binary("value_prefix", value[:min(10, len(value))]))

	// Log what we're about to write to the database
	fmt.Printf("ABOUT TO WRITE TO DB - KEY: %v, VALUE (first 20 bytes): %v\n", key[:8], value[:min(20, len(value))])

	// Buffer the write request with the key as [32]byte
	err := wh.writer.BufferWrite(key, value)
	if err != nil {
		wh.logger.Error("Error writing to database",
			zap.Error(err),
			zap.Binary("key", key[:]))

		// Create an error response using packets.DBResponse
		errorMsg := "Error writing to database"
		dbResp := &packets.DBResponse{
			Status: types.HandlerStatusError,
			Length: uint32(len(errorMsg)),
			Data:   []byte(errorMsg),
		}

		// Encode the response to bytes
		response := dbResp.Encode()
		fmt.Printf("SENDING ERROR RESPONSE: %v\n", response[:min(20, len(response))])
		conn.Send(response)
		return
	}

	// Distribute the record to other nodes if a distributor is available
	if wh.distributor != nil {
		// Use high priority for writes coming from direct client requests
		go func() {
			if err := wh.distributor.DistributeRecord(key, value, PriorityHigh, TargetAll); err != nil {
				wh.logger.Error("Failed to distribute record",
					zap.Error(err),
					zap.Binary("key", key[:]),
				)
			}
		}()
	}

	// Create a success response using packets.DBResponse
	successMsg := "Write successful"
	dbResp := &packets.DBResponse{
		Status: types.HandlerStatusSuccess,
		Length: uint32(len(successMsg)),
		Data:   []byte(successMsg),
	}

	// Encode the response to bytes
	response := dbResp.Encode()

	// Detailed debug of the full response being sent
	fmt.Println("SENDING WRITE SUCCESS RESPONSE", response[:min(20, len(response))], "with status byte:", response[0])
	fmt.Printf("FULL RESPONSE DETAILS:\n  - Status: %d\n  - Length bytes: %v (uint32: %d)\n  - Data: %v\n",
		response[0],
		response[1:5],
		binary.BigEndian.Uint32(response[1:5]),
		response[5:min(25, len(response))])

	// Send the formatted response back to the client
	conn.Send(response)
}
