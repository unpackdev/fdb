package node

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/packets"
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
			Key:   record.Key,
			Value: record.Value,
		}
	}

	// Serialize the batch
	batchData, err := recordBatch.Serialize()
	if err != nil {
		return nil, err
	}

	// Create a network packet with the record batch payload
	networkPacket := packets.NetworkPacket{
		Type:    packets.RecordBatchType,
		Payload: batchData,
	}

	// If account is available, sign the packet
	if d.node.account != nil {
		signedData, err := networkPacket.SerializeWithoutSignature()
		if err != nil {
			return nil, fmt.Errorf("failed to serialize packet for signing: %w", err)
		}

		// Use the account to sign the data directly
		signature, err := d.node.account.Sign(signedData)
		if err != nil {
			return nil, fmt.Errorf("failed to sign record batch packet: %w", err)
		}

		// Set the signature and public key in the network packet
		networkPacket.Signature = signature

		// Get the raw public key data
		pubKeyBytes, err := d.node.account.MarshalPublicKey()
		if err != nil {
			return nil, fmt.Errorf("failed to get public key bytes: %w", err)
		}
		networkPacket.SignaturePubKey = pubKeyBytes
	}

	// Serialize the network packet
	return networkPacket.Serialize()
}

// HandleRecordBatchPacket processes incoming record batches from the network
func (d *P2PDistributor) HandleRecordBatchPacket(ctx context.Context, packet *packets.NetworkPacket, sender peer.ID) error {
	start := time.Now()

	d.logger.Debug(
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

	for _, record := range recordBatch.Records {
		err := d.node.batchWriter.BufferWrite(record.Key, record.Value)
		if err != nil {
			d.logger.Error(
				"Failed to buffer received record",
				zap.Error(err),
				zap.Binary("key_prefix", record.Key[:8]),
			)
		}
	}

	processingTime := time.Since(start)
	d.stats.mu.Lock()
	d.stats.RecordsDistributed += int64(len(recordBatch.Records))
	d.stats.AverageLatencyMs = (d.stats.AverageLatencyMs + processingTime.Milliseconds()) / 2
	d.stats.mu.Unlock()

	d.logger.Debug(
		"Successfully processed record batch",
		zap.String("from_peer", sender.String()),
		zap.Int("record_count", len(recordBatch.Records)),
		zap.Duration("processing_time", processingTime),
	)

	return nil
}
