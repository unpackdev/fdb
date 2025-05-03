package node

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/pkg/db"
	"github.com/unpackdev/fdb/pkg/packets"
	"go.uber.org/zap"
)

// createRecordBatchPacket serializes a batch of records into a network packet
func (d *P2PDistributor) createRecordBatchPacket(records []db.WriteRequest) ([]byte, error) {
	// Convert to a protocol-specific format
	recordBatch := packets.RecordBatch{
		Records: make([]packets.Record, len(records)),
	}

	for i, record := range records {
		recordBatch.Records[i] = packets.Record{
			Key:   record.Key,   // Key is a fixed size array, so it's copied by value
			Value: record.Value, // Use the deep copy to ensure each record has its own value
		}
	}

	// Calculate total payload size for logging
	totalValueSize := 0
	for _, record := range records {
		totalValueSize += len(record.Value)
	}

	d.logger.Debug("Creating record batch packet",
		zap.Int("record_count", len(records)),
		zap.Int("total_value_size", totalValueSize))

	// Serialize the batch
	batchData, err := recordBatch.Serialize()
	if err != nil {
		d.logger.Error("Failed to serialize record batch", zap.Error(err))
		return nil, err
	}

	d.logger.Debug("Serialized record batch",
		zap.Int("serialized_size", len(batchData)))

	// Create a network packet with the record batch payload
	networkPacket := packets.NetworkPacket{
		Type:    packets.RecordBatchType,
		Payload: batchData,
	}

	// If account is available, sign the packet
	if d.node.account != nil {
		signedData, err := networkPacket.SerializeWithoutSignature()
		if err != nil {
			d.logger.Error("Failed to serialize packet for signing", zap.Error(err))
			return nil, fmt.Errorf("failed to serialize packet for signing: %w", err)
		}

		d.logger.Debug("Serialized packet for signing",
			zap.Int("data_to_sign_size", len(signedData)))

		// Use the account to sign the data directly
		signature, err := d.node.account.Sign(signedData)
		if err != nil {
			d.logger.Error("Failed to sign record batch packet", zap.Error(err))
			return nil, fmt.Errorf("failed to sign record batch packet: %w", err)
		}

		d.logger.Debug("Generated signature",
			zap.Int("signature_size", len(signature)))

		// Set the signature and public key in the network packet
		networkPacket.Signature = signature

		// Get the raw public key data
		pubKeyBytes, err := d.node.account.MarshalPublicKey()
		if err != nil {
			d.logger.Error("Failed to marshal public key", zap.Error(err))
			return nil, fmt.Errorf("failed to get public key bytes: %w", err)
		}

		d.logger.Debug("Marshaled public key",
			zap.Int("pubkey_size", len(pubKeyBytes)))

		networkPacket.SignaturePubKey = pubKeyBytes
	}

	// Serialize the network packet
	finalPacket, err := networkPacket.Serialize()
	if err != nil {
		d.logger.Error("Failed to serialize final network packet", zap.Error(err))
		return nil, err
	}

	d.logger.Debug("Final serialized packet",
		zap.Int("final_packet_size", len(finalPacket)),
		zap.Bool("has_signature", networkPacket.Signature != nil),
		zap.Bool("has_pubkey", networkPacket.SignaturePubKey != nil))

	return finalPacket, nil
}

// HandleRecordBatchPacket processes incoming record batches from the network
func (d *P2PDistributor) HandleRecordBatchPacket(ctx context.Context, packet *packets.NetworkPacket, sender peer.ID) error {
	start := time.Now()

	d.logger.Info(
		"Received record batch packet",
		zap.String("from_peer", sender.String()),
		zap.Int("payload_size", len(packet.Payload)),
	)

	// Deserialize the record batch
	recordBatch, err := packets.DeserializeRecordBatch(packet.Payload)
	if err != nil {
		d.logger.Error("Failed to deserialize record batch", zap.Error(err))
		return err
	}

	// Log batch info
	d.logger.Info(
		"Processing incoming record batch",
		zap.String("from_peer", sender.String()),
		zap.Int("record_count", len(recordBatch.Records)),
	)

	// Debug log the first 3 record keys and value prefixes for verification
	for i, record := range recordBatch.Records {
		if i < 3 { // Limit to first 3 records to avoid log spam
			valPrefix := ""
			if len(record.Value) > 0 {
				prefixLen := 10
				if len(record.Value) < prefixLen {
					prefixLen = len(record.Value)
				}
				valPrefix = fmt.Sprintf("%v", record.Value[:prefixLen])
			}
			// Log key and value prefix for this record - use INFO level to ensure visibility
			d.logger.Info(
				"RECEIVED RECORD DETAILS",
				zap.Int("index", i),
				zap.String("key", fmt.Sprintf("%x", record.Key)),
				zap.String("value_prefix", valPrefix),
				zap.Int("value_length", len(record.Value)),
			)
		}
	}

	successCount := 0
	errorCount := 0
	for i, record := range recordBatch.Records {
		// Log every record's key prefix for debugging
		d.logger.Debug(
			"Processing record",
			zap.Int("record_index", i),
			zap.Binary("key_prefix", record.Key[:8]),
		)

		err := d.node.batchWriter.BufferWrite(record.Key, record.Value)
		if err != nil {
			d.logger.Error(
				"Failed to buffer received record",
				zap.Error(err),
				zap.Binary("key_prefix", record.Key[:8]),
				zap.Int("record_index", i),
			)
			errorCount++
		} else {
			successCount++
		}
	}

	// Force a flush of the batchWriter to ensure records are written immediately
	d.node.batchWriter.Flush()

	processingTime := time.Since(start)
	d.stats.mu.Lock()
	d.stats.RecordsDistributed += int64(len(recordBatch.Records))
	d.stats.AverageLatencyMs = (d.stats.AverageLatencyMs + processingTime.Milliseconds()) / 2
	d.stats.mu.Unlock()

	d.logger.Info(
		"Processed record batch",
		zap.String("from_peer", sender.String()),
		zap.Int("record_count", len(recordBatch.Records)),
		zap.Int("success_count", successCount),
		zap.Int("error_count", errorCount),
		zap.Duration("processing_time", processingTime),
	)

	return nil
}
