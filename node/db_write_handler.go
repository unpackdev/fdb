package node

import (
	"fmt"

	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/transports"
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
	fmt.Println("AM I HERE FOR FUCKS SAKES?!")

	// Log packet reception
	wh.logger.Debug("Received write packet",
		zap.String("remote_addr", conn.RemoteAddr()),
		zap.Int("packet_size", len(frame)))
	// Check if first byte is our special marker 0xF0
	forceFlush := false
	offset := 0
	if len(frame) > 0 && frame[0] == 0xF0 {
		forceFlush = true
		offset = 1 // Skip the marker byte
		wh.logger.Debug("Detected force flush marker",
			zap.Int("offset", offset))
	}

	// Adjust minimum length check based on whether we have a marker
	minLen := 34 // 1 byte for action, 32 bytes for key, and at least 1 byte for value
	if forceFlush {
		minLen = 35 // Extra byte for the marker
	}

	if len(frame) < minLen {
		wh.logger.Debug("Invalid message length",
			zap.Int("length", len(frame)),
			zap.Int("expected", minLen))
		conn.Send([]byte{0x01}) // Error code
		return
	}

	// Create a [32]byte key from the frame without using the pool
	var key [32]byte
	copy(key[:], frame[offset+1:offset+33]) // Copy directly from frame, accounting for offset

	// The remaining part is the value (from byte offset+33 onwards)
	value := frame[offset+33+3:]

	// Log key and value details
	wh.logger.Debug("Processing write request",
		zap.Int("offset", offset),
		zap.Binary("key_prefix", key[:8]), // Log first 8 bytes of key for identification
		zap.Int("value_length", len(value)),
		zap.String("value_bytes", string(value)))

	// Buffer the write request with the key as [32]byte
	err := wh.writer.BufferWrite(key, value)
	if err != nil {
		wh.logger.Error("Error writing to database",
			zap.Error(err),
			zap.Binary("key", key[:]))
		conn.Send([]byte{0x01}) // Error code
		return
	}

	// Force flush to ensure data is immediately persisted
	// This is important for reliable testing and client expectations
	wh.logger.Debug("Forcing batch writer flush")
	wh.ForceFlush()
	wh.logger.Debug("Batch writer flush completed")

	// Distribute the record to other nodes if a distributor is available
	if wh.distributor != nil {
		wh.logger.Debug("Starting P2P distribution",
			zap.Binary("key_prefix", key[:8]))
		// Use high priority for writes coming from direct client requests
		go func() {
			if err := wh.distributor.DistributeRecord(key, value, PriorityHigh, TargetAll); err != nil {
				wh.logger.Error("Failed to distribute record",
					zap.Error(err),
					zap.Binary("key", key[:]),
				)
			} else {
				wh.logger.Debug("Successfully queued record for distribution",
					zap.Binary("key_prefix", key[:8]))
			}
		}()
	}

	// Send success response
	conn.Send([]byte{0x00}) // Success code
}
