package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/packets"
	"go.uber.org/zap"
)

// Protocol IDs for P2P distribution
const (
	BulkProtocolSuffix = "/bulk/1.0.0"
)

// Priority represents the importance level of a record batch
type Priority uint8

const (
	PriorityHigh Priority = iota
	PriorityNormal
	PriorityLow
)

// Target specifies the distribution target type
type Target uint8

const (
	TargetAll Target = iota
	TargetValidators
	TargetDirectPeer
)

// DistributionStats tracks performance metrics
type DistributionStats struct {
	RecordsDistributed int64
	BytesDistributed   int64
	BatchesSent        int64
	TransmissionErrors int64
	AverageLatencyMs   int64
	mu                 sync.RWMutex
}

// RecordBatch represents a batch of records to be distributed
type RecordBatch struct {
	Records    []db.WriteRequest
	Priority   Priority
	Target     Target
	TargetPeer *peer.ID // Only set if Target is TargetDirectPeer
}

// P2PDistributor handles efficient record distribution across the network
type P2PDistributor struct {
	node              *Node
	batchSize         int
	bufferPool        *sync.Pool
	stats             *DistributionStats
	highPriorityQueue chan *RecordBatch
	normalQueue       chan *RecordBatch
	lowPriorityQueue  chan *RecordBatch
	ctx               context.Context
	cancel            context.CancelFunc
	logger            logger.Logger
}

// NewP2PDistributor creates a new P2P distributor
func NewP2PDistributor(node *Node, batchSize int) *P2PDistributor {
	ctx, cancel := context.WithCancel(context.Background())

	return &P2PDistributor{
		node:              node,
		batchSize:         batchSize,
		bufferPool:        &sync.Pool{New: func() interface{} { return make([]byte, 64*1024) }},
		stats:             &DistributionStats{},
		highPriorityQueue: make(chan *RecordBatch, 1000),
		normalQueue:       make(chan *RecordBatch, 10000),
		lowPriorityQueue:  make(chan *RecordBatch, 5000),
		ctx:               ctx,
		cancel:            cancel,
		logger:            node.logger,
	}
}

// Start begins processing the distribution queues
func (d *P2PDistributor) Start() {
	d.logger.Info("Starting P2P distributor")

	// Register handlers for incoming P2P record batches via direct protocols
	d.node.network.HandlerRegistry().RegisterHandler(packets.RecordBatchType, d.HandleRecordBatchPacket)
	d.logger.Info("Registered handler for direct RecordBatch messages")

	// Subscribe to PubSub topic for gossip-based record distribution
	// This is critical since most P2P messages are sent via gossip/pubsub
	topic := d.node.network.Topic
	if topic != nil {
		// Create subscription to the PubSub topic
		sub, err := topic.Subscribe()
		if err != nil {
			d.logger.Error("Failed to subscribe to PubSub topic", zap.Error(err))
		} else {
			d.logger.Info("Subscribed to P2P gossip messages")
			// Process subscription messages in a separate goroutine
			go d.handlePubSubMessages(sub)
		}
	} else {
		d.logger.Warn("PubSub topic is nil, gossip distribution will not work")
	}

	// Start queue processors
	go d.processQueue(d.highPriorityQueue, 50*time.Millisecond)  // Process high priority quickly
	go d.processQueue(d.normalQueue, 200*time.Millisecond)       // Process normal priority at medium pace
	go d.processQueue(d.lowPriorityQueue, 1000*time.Millisecond) // Process low priority at slower pace
}

// Stop halts all distribution processes
func (d *P2PDistributor) Stop() {
	d.logger.Info("Stopping P2P distributor")
	d.cancel()
	// Drain queues...
	d.drainQueue(d.highPriorityQueue)
	d.drainQueue(d.normalQueue)
	d.drainQueue(d.lowPriorityQueue)
}

// drainQueue empties a queue without processing items
func (d *P2PDistributor) drainQueue(queue chan *RecordBatch) {
	for {
		select {
		case <-queue:
			// Just discard items
		default:
			return // Queue empty
		}
	}
}

// processQueue handles a specific priority queue
func (d *P2PDistributor) processQueue(queue chan *RecordBatch, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case batch := <-queue:
			d.distributeBatch(batch)
		case <-ticker.C:
			// Check if queue has accumulated items for batching
			d.processPendingBatches(queue)
		}
	}
}

// processPendingBatches attempts to combine and process multiple batches
func (d *P2PDistributor) processPendingBatches(queue chan *RecordBatch) {
	// Skip if queue is empty
	if len(queue) == 0 {
		return
	}

	// Process up to batchSize records or empty the queue
	recordCount := 0
	var combinedBatch RecordBatch
	combinedBatch.Records = make([]db.WriteRequest, 0, d.batchSize)

	// Try to empty queue while keeping target and priority consistent
	drainCount := 0
	maxDrain := 100 // Safety limit

	for recordCount < d.batchSize && drainCount < maxDrain {
		select {
		case batch := <-queue:
			// If this is the first batch, set the target type
			if drainCount == 0 {
				combinedBatch.Target = batch.Target
				combinedBatch.Priority = batch.Priority
				combinedBatch.TargetPeer = batch.TargetPeer
			}

			// Only combine batches going to the same target type
			if combinedBatch.Target == batch.Target &&
				(combinedBatch.TargetPeer == nil || batch.TargetPeer == nil ||
					*combinedBatch.TargetPeer == *batch.TargetPeer) {

				spaceLeft := d.batchSize - recordCount
				recordsToAdd := len(batch.Records)

				if recordsToAdd > spaceLeft {
					// Take only what fits
					combinedBatch.Records = append(combinedBatch.Records, batch.Records[:spaceLeft]...)

					// Put the rest back in the queue
					batch.Records = batch.Records[spaceLeft:]
					select {
					case queue <- batch:
						// Successfully put back remainder
					default:
						// Queue full, just log and continue
						d.logger.Warn("Failed to requeue partial batch - queue full")
					}

					recordCount += spaceLeft
				} else {
					// All fits, add everything
					combinedBatch.Records = append(combinedBatch.Records, batch.Records...)
					recordCount += recordsToAdd
				}
			} else {
				// Different target, put it back
				select {
				case queue <- batch:
					// Successfully put back
				default:
					// Queue full, just log
					d.logger.Warn("Failed to requeue incompatible batch - queue full")
				}
			}

			drainCount++
		default:
			// Queue empty
			break
		}
	}

	// Process the combined batch if we got any records
	if recordCount > 0 {
		d.distributeBatch(&combinedBatch)
	}
}

// distributeBatch sends a batch of records according to its target
func (d *P2PDistributor) distributeBatch(batch *RecordBatch) {
	if len(batch.Records) == 0 {
		return
	}

	switch batch.Target {
	case TargetAll:
		d.distributeToAllPeers(batch)
	case TargetValidators:
		d.distributeToValidators(batch)
	case TargetDirectPeer:
		if batch.TargetPeer != nil {
			d.distributeToPeer(*batch.TargetPeer, batch.Records)
		} else {
			d.logger.Error("Cannot distribute to direct peer: no peer specified")
		}
	}
}

// DistributeRecord adds a record to the appropriate distribution queue
func (d *P2PDistributor) DistributeRecord(key [32]byte, value []byte, priority Priority, target Target) error {
	// Create a single-record batch
	record := db.WriteRequest{
		Key:   key,
		Value: value,
	}

	batch := &RecordBatch{
		Records:  []db.WriteRequest{record},
		Priority: priority,
		Target:   target,
	}

	// Queue based on priority
	var queue chan *RecordBatch
	switch priority {
	case PriorityHigh:
		queue = d.highPriorityQueue
	case PriorityNormal:
		queue = d.normalQueue
	case PriorityLow:
		queue = d.lowPriorityQueue
	}

	// Non-blocking send to avoid caller being blocked
	select {
	case queue <- batch:
		return nil
	default:
		// Queue full, handle with fallback strategy
		go func() {
			// This will block in its own goroutine
			queue <- batch
		}()
		return nil
	}
}

// DistributeRecordToPeer sends a record directly to a specific peer
func (d *P2PDistributor) DistributeRecordToPeer(key [32]byte, value []byte, peerID peer.ID, priority Priority) error {
	record := db.WriteRequest{
		Key:   key,
		Value: value,
	}

	batch := &RecordBatch{
		Records:    []db.WriteRequest{record},
		Priority:   priority,
		Target:     TargetDirectPeer,
		TargetPeer: &peerID,
	}

	// Queue based on priority
	var queue chan *RecordBatch
	switch priority {
	case PriorityHigh:
		queue = d.highPriorityQueue
	case PriorityNormal:
		queue = d.normalQueue
	case PriorityLow:
		queue = d.lowPriorityQueue
	}

	// Non-blocking send with fallback
	select {
	case queue <- batch:
		return nil
	default:
		go func() {
			queue <- batch
		}()
		return nil
	}
}

// distributeToAllPeers sends records to all connected peers
func (d *P2PDistributor) distributeToAllPeers(batch *RecordBatch) {
	// For smaller data, use gossip protocol
	if isSmallBatch(batch) {
		d.distributeViaGossip(batch)
		return
	}

	// For larger batches, use direct connections to each peer
	peers := d.node.network.Host().Network().Peers()
	if len(peers) == 0 {
		d.logger.Debug("No peers to distribute records to")
		return
	}

	for _, peer := range peers {
		// Skip self
		if peer == d.node.network.Host().ID() {
			continue
		}

		// Send in parallel but with throttling
		go d.distributeToPeer(peer, batch.Records)
	}
}

// isSmallBatch determines if a batch is small enough for gossip protocol
func isSmallBatch(batch *RecordBatch) bool {
	totalSize := 0
	for _, record := range batch.Records {
		// Rough size estimation: 32 bytes for key + value length
		totalSize += 32 + len(record.Value)
		if totalSize > 4096 {
			return false // Batch too large for gossip
		}
	}
	return len(batch.Records) < 100 // Limit record count too
}

// distributeViaGossip sends records using the PubSub system
func (d *P2PDistributor) distributeViaGossip(batch *RecordBatch) {
	// Serialize the batch
	packet, err := d.createRecordBatchPacket(batch.Records)
	if err != nil {
		d.logger.Error("Failed to create record batch packet", zap.Error(err))
		return
	}

	// Publish to topic
	err = d.node.network.BroadcastMessage(packet)
	if err != nil {
		d.logger.Error("Failed to broadcast record batch", zap.Error(err))
		d.stats.mu.Lock()
		d.stats.TransmissionErrors++
		d.stats.mu.Unlock()
		return
	}

	// Update stats
	d.stats.mu.Lock()
	d.stats.RecordsDistributed += int64(len(batch.Records))
	d.stats.BatchesSent++
	d.stats.BytesDistributed += int64(len(packet))
	d.stats.mu.Unlock()
}

// distributeToValidators sends records only to validator nodes
func (d *P2PDistributor) distributeToValidators(batch *RecordBatch) {
	// TODO: Implement validator peer filtering
	// For now, distribute to all peers as fallback
	d.distributeToAllPeers(batch)
}

// distributeToPeer sends a batch of records to a specific peer
func (d *P2PDistributor) distributeToPeer(targetPeer peer.ID, records []db.WriteRequest) {
	// Skip if peer is self
	if targetPeer == d.node.network.Host().ID() {
		return
	}

	// Create the packet
	packet, err := d.createRecordBatchPacket(records)
	if err != nil {
		d.logger.Error("Failed to create record batch packet", zap.Error(err))
		return
	}

	// Choose protocol based on batch size
	protocolID := d.node.network.ProtocolID
	if len(packet) > 100*1024 {
		// For large batches, use the bulk protocol
		bulkProtocolID := protocol.ID(string(protocolID) + BulkProtocolSuffix)
		err = d.node.network.SendToPeer(d.ctx, targetPeer, bulkProtocolID, packet)
	} else {
		// For smaller batches, use the standard protocol
		err = d.node.network.SendMessage(d.ctx, protocolID, targetPeer, packet)
	}

	if err != nil {
		d.logger.Error(
			"Failed to send record batch to peer",
			zap.String("peer", targetPeer.String()),
			zap.Error(err),
		)
		d.stats.mu.Lock()
		d.stats.TransmissionErrors++
		d.stats.mu.Unlock()
		return
	}

	// Update stats
	d.stats.mu.Lock()
	d.stats.RecordsDistributed += int64(len(records))
	d.stats.BatchesSent++
	d.stats.BytesDistributed += int64(len(packet))
	d.stats.mu.Unlock()
}

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

	// Log receipt
	d.logger.Debug("Received record batch packet",
		zap.String("from_peer", sender.String()),
		zap.Int("payload_size", len(packet.Payload)))

	// Deserialize the record batch
	recordBatch, err := packets.DeserializeRecordBatch(packet.Payload)
	if err != nil {
		d.logger.Error("Failed to deserialize record batch", zap.Error(err))
		return err
	}

	// Log batch info
	d.logger.Info("Processing incoming record batch",
		zap.String("from_peer", sender.String()),
		zap.Int("record_count", len(recordBatch.Records)))

	// Buffer directly into the node's writer
	for _, record := range recordBatch.Records {
		// Skip empty records
		if len(record.Value) == 0 {
			continue
		}

		err := d.node.batchWriter.BufferWrite(record.Key, record.Value)
		if err != nil {
			d.logger.Error("Failed to buffer received record",
				zap.Error(err),
				zap.Binary("key_prefix", record.Key[:8]))
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

// handlePubSubMessages processes incoming PubSub messages from the gossip network
func (d *P2PDistributor) handlePubSubMessages(sub *pubsub.Subscription) {
	d.logger.Info("Starting PubSub message handler")

	for {
		select {
		case <-d.ctx.Done():
			d.logger.Info("Stopping PubSub message handler")
			return
		default:
			// Receive messages from the subscription
			msg, err := sub.Next(d.ctx)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return // Context canceled, exit gracefully
				}
				d.logger.Error("Error receiving PubSub message", zap.Error(err))
				continue
			}

			// Skip messages from self
			if msg.ReceivedFrom == d.node.network.Host().ID() {
				d.logger.Debug("Skipping PubSub message from self")
				continue
			}

			d.logger.Debug("Received PubSub message",
				zap.String("from", msg.ReceivedFrom.String()),
				zap.Int("data_size", len(msg.Data)))

			// Deserialize the network packet
			networkPacket, err := packets.DeserializeNetworkPacket(msg.Data)
			if err != nil {
				d.logger.Error("Failed to deserialize PubSub network packet", zap.Error(err))
				continue
			}

			// Process only RecordBatch type packets
			if networkPacket.Type == packets.RecordBatchType {
				// Handle the record batch packet using our existing handler
				err = d.HandleRecordBatchPacket(d.ctx, networkPacket, msg.ReceivedFrom)
				if err != nil {
					d.logger.Error("Failed to process PubSub RecordBatch", zap.Error(err))
				}
			} else {
				d.logger.Debug("Ignoring non-RecordBatch PubSub message",
					zap.String("packet_type", networkPacket.Type.String()))
			}
		}
	}
}

// GetStats returns the current distribution statistics
func (d *P2PDistributor) GetStats() DistributionStats {
	d.stats.mu.RLock()
	defer d.stats.mu.RUnlock()

	// Return a copy to avoid race conditions
	return DistributionStats{
		RecordsDistributed: d.stats.RecordsDistributed,
		BytesDistributed:   d.stats.BytesDistributed,
		BatchesSent:        d.stats.BatchesSent,
		TransmissionErrors: d.stats.TransmissionErrors,
		AverageLatencyMs:   d.stats.AverageLatencyMs,
	}
}
