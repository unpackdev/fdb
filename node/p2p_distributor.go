package node

import (
	"context"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/db"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/packets"
	"github.com/unpackdev/fdb/state"
	"go.uber.org/zap"
)

// Protocol IDs for P2P distribution
const (
	BulkProtocolSuffix = "/bulk/1.0.0"
)

// P2PDistributorStateType is the type identifier for P2PDistributor state
const P2PDistributorStateType state.StateType = "p2p_distributor"

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
	mu                 deadlock.RWMutex
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
	stateMgr          *state.StateManager // Manages the distributor's state
}

// NewP2PDistributor creates a new P2P distributor
func NewP2PDistributor(node *Node, batchSize int) *P2PDistributor {
	ctx, cancel := context.WithCancel(context.Background())

	node.stateMgr.SetState(P2PDistributorStateType, state.Initializing)
	defer node.stateMgr.SetState(P2PDistributorStateType, state.Initialized)

	return &P2PDistributor{
		node:              node,
		batchSize:         batchSize,
		bufferPool:        &sync.Pool{New: func() any { return make([]byte, 32*1024*1024) }}, // 32MB buffer to handle large payloads
		stats:             &DistributionStats{},
		highPriorityQueue: make(chan *RecordBatch, 1000),
		normalQueue:       make(chan *RecordBatch, 10000),
		lowPriorityQueue:  make(chan *RecordBatch, 5000),
		ctx:               ctx,
		cancel:            cancel,
		logger:            node.logger,
		stateMgr:          node.stateMgr,
	}
}

// Start begins processing the distribution queues
func (d *P2PDistributor) Start() error {
	d.logger.Info("Starting P2P distributor")
	d.stateMgr.SetState(P2PDistributorStateType, state.Starting)

	// Register handlers for incoming P2P record batches via direct protocols
	d.node.network.HandlerRegistry().RegisterHandler(packets.RecordBatchType, d.HandleRecordBatchPacket)
	d.logger.Info("Registered handler for direct RecordBatch messages")

	// Register a special handler for the bulk protocol (used for large transfers)
	bulkProtocolID := protocol.ID(string(d.node.network.ProtocolID) + BulkProtocolSuffix)
	d.node.network.Host().SetStreamHandler(bulkProtocolID, d.HandleBulkStream)
	d.logger.Info("Registered handler for bulk transfers", zap.String("protocol", string(bulkProtocolID)))

	// Create subscription to the PubSub topic
	sub, err := d.node.network.Topic.Subscribe()
	if err != nil {
		d.logger.Error("Failed to subscribe to PubSub topic", zap.Error(err))
		d.stateMgr.SetState(P2PDistributorStateType, state.Failed)
		return err
	} else {
		d.logger.Info("Subscribed to P2P gossip messages")
		// Process subscription messages in a separate goroutine
		go d.handlePubSubMessages(sub)
	}

	// Start queue processors with optimized intervals for better batching
	// Fast enough for high priority, but still allows some batching
	go d.processQueue(d.highPriorityQueue, 25*time.Millisecond)

	// This is critical for the normal queue which handles the bulk of P2P distribution
	// We reduce the interval to match better with how the test writes records rapidly
	go d.processQueue(d.normalQueue, 30*time.Millisecond)

	// Longer interval for low priority to maximize batching
	go d.processQueue(d.lowPriorityQueue, 100*time.Millisecond)

	d.logger.Info("Started queue processors with optimized batching intervals",
		zap.Duration("high_priority", 25*time.Millisecond),
		zap.Duration("normal_priority", 30*time.Millisecond),
		zap.Duration("low_priority", 100*time.Millisecond))

	d.stateMgr.SetState(P2PDistributorStateType, state.Started)
	return nil
}

// Stop gracefully halts all distribution processes
func (d *P2PDistributor) Stop() {
	d.logger.Info("Stopping P2P distributor - processing remaining items")
	d.stateMgr.SetState(P2PDistributorStateType, state.Stopping)

	// Process all remaining items in queues before shutting down
	// This ensures data consistency and prevents data loss
	highCount := d.processRemainingQueue(d.highPriorityQueue)
	normalCount := d.processRemainingQueue(d.normalQueue)
	lowCount := d.processRemainingQueue(d.lowPriorityQueue)

	// Log how many items were processed during shutdown
	d.logger.Info("Completed processing remaining items during shutdown",
		zap.Int("high_priority_processed", highCount),
		zap.Int("normal_priority_processed", normalCount),
		zap.Int("low_priority_processed", lowCount))

	// Signal all goroutines to terminate after queue processing
	d.cancel()

	d.stateMgr.SetState(P2PDistributorStateType, state.Stopped)
}

// processRemainingQueue processes all remaining items in the queue before shutdown
// Returns the count of processed items
func (d *P2PDistributor) processRemainingQueue(queue chan *RecordBatch) int {
	count := 0

	// Use a timeout to avoid blocking forever during shutdown
	timeout := time.After(5 * time.Second) // Longer timeout to allow for processing

	for {
		select {
		case batch := <-queue:
			if batch == nil {
				continue
			}

			// Process this batch (write to database)
			d.logger.Debug("Processing queued batch during shutdown",
				zap.Int("record_count", len(batch.Records)))

			// Write directly to DB using optimized BatchWriter
			batchWriter := d.node.batchWriter
			for _, record := range batch.Records {
				if err := batchWriter.BufferWrite(record.Key, record.Value); err != nil {
					d.logger.Error(
						"Failed to write record during shutdown",
						zap.Error(err),
						zap.ByteString("key", record.Key[:]),
					)
				}
			}

			count++

		case <-timeout:
			// Return if we've been trying for too long
			d.logger.Warn("Shutdown processing timed out, some items may not be processed")
			return count

		default:
			// Queue is empty, all items processed
			return count
		}
	}
}

// processQueue handles a specific priority queue
func (d *P2PDistributor) processQueue(queue chan *RecordBatch, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Buffered accumulator for batches that should be processed together
	var pendingBatches []*RecordBatch

	// Distribute any accumulated batches
	distributeAccumulated := func() {
		// Nothing to distribute
		if len(pendingBatches) == 0 {
			return
		}

		// Count total records in pending batches for logging
		totalPendingRecords := 0
		for _, b := range pendingBatches {
			totalPendingRecords += len(b.Records)
		}

		d.logger.Debug("Distributing accumulated batches",
			zap.Int("batch_count", len(pendingBatches)),
			zap.Int("total_records", totalPendingRecords))

		// If there's just one batch and it's not full, process it directly
		if len(pendingBatches) == 1 && len(pendingBatches[0].Records) < d.batchSize {
			d.logger.Debug("Distributing single non-full batch directly",
				zap.Int("record_count", len(pendingBatches[0].Records)))
			d.distributeBatch(pendingBatches[0])
			pendingBatches = nil
			return
		}

		// Multiple batches - put them back in the queue and let processPendingBatches
		// handle the optimal batching
		d.logger.Debug("Requeuing multiple batches for optimized batching",
			zap.Int("batch_count", len(pendingBatches)),
			zap.Int("total_records", totalPendingRecords))

		for _, batch := range pendingBatches {
			// Use blocking send to ensure no records are dropped
			queue <- batch
		}

		// Process with the batching logic
		d.processPendingBatches(queue)
		pendingBatches = nil
	}

	for {
		select {
		case <-d.ctx.Done():
			// Process any remaining batches before shutting down
			d.logger.Debug("Processing remaining batches before shutdown",
				zap.Int("pending_batch_count", len(pendingBatches)))
			distributeAccumulated()
			return

		case batch := <-queue:
			// Add to the accumulator rather than immediately distributing
			pendingBatches = append(pendingBatches, batch)

			// Log the current accumulation state
			totalRecords := 0
			for _, b := range pendingBatches {
				totalRecords += len(b.Records)
			}

			d.logger.Debug("Added batch to accumulator",
				zap.Int("batch_records", len(batch.Records)),
				zap.Int("accumulated_batches", len(pendingBatches)),
				zap.Int("accumulated_records", totalRecords))

			// If our accumulator would exceed the batch size, trigger distribution
			if totalRecords >= d.batchSize {
				d.logger.Debug("Accumulated records meet or exceed batch size, triggering distribution",
					zap.Int("accumulated_records", totalRecords),
					zap.Int("batch_size", d.batchSize))
				distributeAccumulated()
			}

		case <-ticker.C:
			// Only log if we have pending batches
			if len(pendingBatches) > 0 {
				totalRecords := 0
				for _, b := range pendingBatches {
					totalRecords += len(b.Records)
				}

				d.logger.Debug("Ticker triggered distribution of accumulated batches",
					zap.Int("accumulated_batches", len(pendingBatches)),
					zap.Int("accumulated_records", totalRecords))
			}

			// Check if queue has accumulated items for batching
			distributeAccumulated()

			// Also check if there are more items in the channel that could be batched
			if len(queue) > 0 {
				d.logger.Debug("Additional batches in queue, processing",
					zap.Int("queue_length", len(queue)))
				d.processPendingBatches(queue)
			}
		}
	}
}

// processPendingBatches attempts to combine and process multiple batches
func (d *P2PDistributor) processPendingBatches(queue chan *RecordBatch) {
	queueLen := len(queue)
	if queueLen == 0 {
		return
	}

	d.logger.Debug("Starting to process pending batches",
		zap.Int("queue_length", queueLen),
		zap.Int("batch_size", d.batchSize))

	// Process up to batchSize records or empty the queue
	recordCount := 0
	var combinedBatch RecordBatch
	combinedBatch.Records = make([]db.WriteRequest, 0, d.batchSize)

	// Try to empty queue while keeping target and priority consistent
	drainCount := 0
	maxDrain := 10000 // Safety limit

	batchesDrained := 0
	recordsCombined := 0
	batchesRequeued := 0

	d.logger.Debug("Starting drain loop for batch combination")

drainLoop:
	for recordCount < d.batchSize && drainCount < maxDrain {
		select {
		case batch := <-queue:
			batchesDrained++
			d.logger.Debug("Drained batch from queue",
				zap.Int("records_in_batch", len(batch.Records)),
				zap.Uint8("priority", uint8(batch.Priority)),
				zap.Uint8("target", uint8(batch.Target)))

			// If this is the first batch, set the target type
			if drainCount == 0 {
				combinedBatch.Target = batch.Target
				combinedBatch.Priority = batch.Priority
				combinedBatch.TargetPeer = batch.TargetPeer
				d.logger.Debug("Set combined batch parameters from first batch",
					zap.Uint8("target", uint8(combinedBatch.Target)),
					zap.Uint8("priority", uint8(combinedBatch.Priority)))
			}

			// Only combine batches going to the same target type
			if combinedBatch.Target == batch.Target &&
				(combinedBatch.TargetPeer == nil || batch.TargetPeer == nil ||
					*combinedBatch.TargetPeer == *batch.TargetPeer) {

				spaceLeft := d.batchSize - recordCount
				recordsToAdd := len(batch.Records)

				d.logger.Debug("Batch compatible with combined batch",
					zap.Int("records_to_add", recordsToAdd),
					zap.Int("space_left", spaceLeft))

				if recordsToAdd > spaceLeft {
					// Take only what fits
					combinedBatch.Records = append(combinedBatch.Records, batch.Records[:spaceLeft]...)

					// Put the rest back in the queue
					batch.Records = batch.Records[spaceLeft:]
					// Use blocking operation to ensure no records are dropped
					queue <- batch
					batchesRequeued++

					recordsCombined += spaceLeft
					recordCount += spaceLeft
					d.logger.Debug("Partial batch combined - remaining records requeued",
						zap.Int("records_added", spaceLeft),
						zap.Int("records_requeued", len(batch.Records)))
				} else {
					// All fits, add everything
					combinedBatch.Records = append(combinedBatch.Records, batch.Records...)
					recordsCombined += recordsToAdd
					recordCount += recordsToAdd
					d.logger.Debug("Full batch combined",
						zap.Int("records_added", recordsToAdd),
						zap.Int("current_combined_count", recordCount))
				}
			} else {
				// Different target, put it back
				// Use blocking operation to ensure no records are dropped
				queue <- batch
				batchesRequeued++
				d.logger.Debug("Incompatible batch requeued",
					zap.Uint8("batch_target", uint8(batch.Target)),
					zap.Uint8("combined_target", uint8(combinedBatch.Target)))
			}

			drainCount++
		default:
			// Queue empty, break out of the outer loop
			break drainLoop
		}
	}

	// Process the combined batch if we got any records
	if recordCount > 0 {
		d.distributeBatch(&combinedBatch)
	}
}

// distributeBatch sends a batch of records according to its target
func (d *P2PDistributor) distributeBatch(batch *RecordBatch) {
	recordCount := len(batch.Records)

	if recordCount == 0 {
		return
	}

	d.logger.Debug("Distributing batch",
		zap.Int("record_count", recordCount),
		zap.Uint8("target", uint8(batch.Target)),
		zap.Uint8("priority", uint8(batch.Priority)))

	switch batch.Target {
	case TargetAll:
		d.logger.Debug("Distributing to all peers", zap.Int("record_count", recordCount))
		d.distributeToAllPeers(batch)
	case TargetValidators:
		d.logger.Debug("Distributing to validators", zap.Int("record_count", recordCount))
		d.distributeToValidators(batch)
	case TargetDirectPeer:
		if batch.TargetPeer != nil {
			d.logger.Debug("Distributing to direct peer",
				zap.Int("record_count", recordCount),
				zap.String("peer_id", batch.TargetPeer.String()))
			d.distributeToPeer(*batch.TargetPeer, batch.Records)
		} else {
			d.logger.Error("Cannot distribute to direct peer: no peer specified")
		}
	}
}

// DistributeRecord adds a record to the appropriate distribution queue
func (d *P2PDistributor) DistributeRecord(key [32]byte, value []byte, priority Priority, target Target) error {
	// Create a deep copy of the value when a record first enters the system
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	record := db.WriteRequest{
		Key:   key,
		Value: valueCopy,
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

	// Use a timeout to avoid blocking forever while still being reliable
	timeoutCtx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
	defer cancel()

	select {
	case queue <- batch:
		return nil
	case <-timeoutCtx.Done():
		// If we time out, still try to send in a separate goroutine
		go func() {
			select {
			case queue <- batch:
				// Successfully queued
			case <-d.ctx.Done():
				// System shutting down, log the loss
				d.logger.Error("Lost record during shutdown - queue operation timed out")
			}
		}()
		return errors.New("queue operation timed out, record queued in background")
	}
}

// DistributeRecordToPeer sends a record directly to a specific peer
func (d *P2PDistributor) DistributeRecordToPeer(key [32]byte, value []byte, peerID peer.ID, priority Priority) error {
	// Create a deep copy of the value when a record first enters the system
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	record := db.WriteRequest{
		Key:   key,
		Value: valueCopy,
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

	// Use a timeout to avoid blocking forever while still being reliable
	timeoutCtx, cancel := context.WithTimeout(d.ctx, 500*time.Millisecond)
	defer cancel()

	select {
	case queue <- batch:
		return nil
	case <-timeoutCtx.Done():
		// If we time out, still try to send in a separate goroutine
		go func() {
			select {
			case queue <- batch:
				// Successfully queued
			case <-d.ctx.Done():
				// System shutting down, log the loss
				d.logger.Error("Lost direct peer record during shutdown - queue operation timed out")
			}
		}()
		return errors.New("queue operation timed out, direct peer record queued in background")
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
	if len(packet) > 4*1024 {
		// For batches larger than 4KB, use the bulk protocol
		d.logger.Debug("Using bulk protocol for large packet",
			zap.Int("packet_size", len(packet)),
			zap.String("peer", targetPeer.String()))
		bulkProtocolID := protocol.ID(string(protocolID) + BulkProtocolSuffix)
		err = d.node.network.SendToPeer(d.ctx, targetPeer, bulkProtocolID, packet)
	} else {
		// For smaller batches, use the standard protocol
		d.logger.Debug("Using standard protocol for small packet",
			zap.Int("packet_size", len(packet)),
			zap.String("peer", targetPeer.String()))
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

// handlePubSubMessages processes incoming PubSub messages from the gossip network
func (d *P2PDistributor) handlePubSubMessages(sub *pubsub.Subscription) {
	d.logger.Info("Starting PubSub message handler")
	messageCount := 0

	for {
		select {
		case <-d.ctx.Done():
			d.logger.Info("Stopping PubSub message handler", zap.Int("total_messages_processed", messageCount))
			return
		default:
			msg, err := sub.Next(d.ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					d.logger.Info("Context canceled, exiting PubSub handler", zap.Int("total_messages_processed", messageCount))
					return // Context canceled, exit gracefully
				}

				d.logger.Error("Error receiving PubSub message", zap.Error(err))
				continue
			}

			messageCount++

			if msg.ReceivedFrom == d.node.network.Host().ID() {
				d.logger.Debug("Skipping PubSub message from self")
				continue
			}

			d.logger.Info(
				"Received PubSub message",
				zap.String("from", msg.ReceivedFrom.String()),
				zap.Int("data_size", len(msg.Data)),
				zap.Int("message_count", messageCount),
			)

			networkPacket, err := packets.DeserializeNetworkPacket(msg.Data)
			if err != nil {
				d.logger.Error("Failed to deserialize PubSub network packet", zap.Error(err))
				continue
			}

			d.logger.Info(
				"Deserialized network packet",
				zap.String("from", msg.ReceivedFrom.String()),
				zap.Int("packet_type", int(networkPacket.Type)),
				zap.Int("payload_size", len(networkPacket.Payload)),
			)

			switch networkPacket.Type {
			case packets.RecordBatchType:
				// Handle the record batch packet using our existing handler
				d.logger.Info("Processing RecordBatch from PubSub",
					zap.String("sender", msg.ReceivedFrom.String()))

				err = d.HandleRecordBatchPacket(d.ctx, networkPacket, msg.ReceivedFrom)
				if err != nil {
					d.logger.Error("Failed to process PubSub RecordBatch", zap.Error(err))
				} else {
					d.logger.Info("Successfully processed PubSub RecordBatch",
						zap.String("sender", msg.ReceivedFrom.String()))
				}
			default:
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
