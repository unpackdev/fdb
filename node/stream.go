package node

import (
	"bufio"
	"bytes"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/unpackdev/fdb/packets"
	"go.uber.org/zap"
)

// HandleBulkStream processes large data streams sent through the bulk protocol
// It bypasses signature verification for performance reasons when handling large payloads
func (d *P2PDistributor) HandleBulkStream(s network.Stream) {
	peerID := s.Conn().RemotePeer()
	d.logger.Info("Received bulk stream from peer", zap.String("peer_id", peerID.String()))

	// Use a large buffer size (32MB) to handle very large payloads
	reader := bufio.NewReaderSize(s, 32*1024*1024)

	// Read all data from the stream into a buffer with more robust handling
	var buffer bytes.Buffer
	total, err := io.Copy(&buffer, reader)
	if err != nil && err != io.EOF {
		d.logger.Error("Failed to read from bulk stream", zap.Error(err), zap.String("peerID", peerID.String()))
		s.Close()
		return
	}

	// Get the complete message
	message := buffer.Bytes()
	d.logger.Debug("Received bulk packet",
		zap.Int64("bytes_read", total),
		zap.Int("buffer_size", len(message)),
		zap.String("peer_id", peerID.String()))

	startTime := time.Now()

	// Deserialize the NetworkPacket directly without verification
	networkPacket, npErr := packets.DeserializeNetworkPacket(message)
	if npErr != nil {
		d.logger.Error("Failed to deserialize bulk network packet", zap.Error(npErr))
		s.Close()
		return
	}

	d.logger.Debug("Received bulk message",
		zap.String("from_peer", peerID.String()),
		zap.String("packet_type", networkPacket.Type.String()),
		zap.Int("payload_size", len(networkPacket.Payload)),
	)

	// Process the packet depending on its type - for now just handle record batches
	if networkPacket.Type == packets.RecordBatchType {
		if err := d.HandleRecordBatchPacket(d.ctx, networkPacket, peerID); err != nil {
			d.logger.Error("Failed to handle bulk record batch packet", zap.Error(err))
		} else {
			d.logger.Debug("Successfully processed bulk record batch packet",
				zap.Int("payload_size", len(networkPacket.Payload)),
				zap.Duration("processing_time", time.Since(startTime)))
		}
	} else {
		d.logger.Warn("Received unknown packet type via bulk protocol",
			zap.String("type", networkPacket.Type.String()),
			zap.String("peer_id", peerID.String()))
	}

	s.Close()
}
