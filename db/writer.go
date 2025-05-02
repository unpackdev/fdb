package db

import (
	"context"
	"sync"
	"time"

	"github.com/erigontech/mdbx-go/mdbx"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/observability"
	"go.uber.org/zap"
)

// WriteRequest represents a key-value pair to be written to the database.
type WriteRequest struct {
	Key   [32]byte // Fixed-size byte array for keys
	Value []byte   // Value as byte slice
}

// BatchWriter handles batch writes with concurrency support and multiple workers.
type BatchWriter struct {
	db             *Db
	workerChannels []chan WriteRequest   // Dedicated channel for each worker
	workerBuffers  []map[[32]byte][]byte // Separate buffer for each worker using fixed-size byte arrays for keys
	workerMutexes  []sync.Mutex          // Separate mutex for each worker
	maxBatchSize   int                   // Max size of the batch before flush
	flushInterval  time.Duration         // Time interval for auto-flush
	stopChannel    chan struct{}         // Channel to signal the background workers to stop
	workers        int                   // Number of worker goroutines
	
	// Added for metrics - these won't affect core functionality
	metrics        *BatchWriterMetrics   // Optional metrics for monitoring performance
	ctx            context.Context       // Context for metrics
}

// NewBatchWriter initializes a BatchWriter with a configurable number of workers.
func NewBatchWriter(db *Db, maxBatchSize int, flushInterval time.Duration, workers int) *BatchWriter {
	// Try to initialize metrics - if this fails, we'll continue without metrics
	ctx := context.Background()
	var metricsInstance *BatchWriterMetrics
	if observability.G() != nil { // Only if observability is initialized
		var err error
		metricsInstance, err = InitializeBatchWriterMetrics(ctx, observability.G().Meter)
		if err != nil {
			// If metrics initialization fails, log it but continue without metrics
			zap.L().Debug("Failed to initialize BatchWriter metrics, continuing without metrics", zap.Error(err))
			metricsInstance = nil
		}
	}
	bw := &BatchWriter{
		db:             db,
		workerChannels: make([]chan WriteRequest, workers),
		workerBuffers:  make([]map[[32]byte][]byte, workers),
		workerMutexes:  make([]sync.Mutex, workers),
		maxBatchSize:   maxBatchSize,
		flushInterval:  flushInterval,
		stopChannel:    make(chan struct{}),
		workers:        workers,
		metrics:        metricsInstance, // This may be nil if metrics initialization failed
		ctx:            ctx,
	}

	// Initialize each worker's channel and buffer
	for i := 0; i < workers; i++ {
		bw.workerChannels[i] = make(chan WriteRequest, 500000) // Dedicated buffered channel for each worker
		bw.workerBuffers[i] = make(map[[32]byte][]byte)
		go bw.runWorker(i)
	}
	
	// Initialize worker metrics if available
	if bw.metrics != nil {
		// Initialize worker buffer metrics
		if err := bw.metrics.InitializeWorkerBufferMetrics(ctx, observability.G().Meter, workers); err != nil {
			zap.L().Debug("Failed to initialize worker buffer metrics", zap.Error(err))
		}
		
		// Record active workers
		bw.metrics.ActiveWorkers.Add(ctx, int64(workers))
	}

	return bw
}

// runWorker is a background goroutine that listens for write requests and flushes the buffer.
func (bw *BatchWriter) runWorker(workerID int) {
	ticker := time.NewTicker(bw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case req := <-bw.workerChannels[workerID]:
			// Record queue depth metrics if enabled
			if bw.metrics != nil {
				bw.metrics.UpdateQueueDepth(bw.ctx, -1) // Decrement queue depth
			}
			
			bw.workerMutexes[workerID].Lock()
			// Add the request to the worker's buffer
			bw.workerBuffers[workerID][req.Key] = req.Value
			
			// Update metrics for buffer size if enabled
			if bw.metrics != nil {
				bw.metrics.UpdateWorkerBuffer(bw.ctx, workerID, int64(len(bw.workerBuffers[workerID])))
			}

			// Check if buffer exceeds max size, then flush
			if len(bw.workerBuffers[workerID]) >= bw.maxBatchSize {
				bw.flush(workerID)
			}
			bw.workerMutexes[workerID].Unlock()

		case <-ticker.C:
			// Periodic flush based on time interval
			bw.workerMutexes[workerID].Lock()
			bw.flush(workerID)
			bw.workerMutexes[workerID].Unlock()

		case <-bw.stopChannel:
			// On stop signal, flush remaining data
			bw.workerMutexes[workerID].Lock()
			bw.flush(workerID)
			bw.workerMutexes[workerID].Unlock()
			return
		}
	}
}

// BufferWrite adds a key-value pair to the batch and writes it to the worker's dedicated channel.
func (bw *BatchWriter) BufferWrite(key [32]byte, value []byte) error {
	// Use a more sophisticated hash distribution for better worker assignment
	// XOR the first and last bytes for slightly better distribution
	workerID := int(key[0]^key[31]) % bw.workers
	
	// Update queue depth metric if metrics are enabled
	if bw.metrics != nil {
		bw.metrics.UpdateQueueDepth(bw.ctx, 1) // Increment queue depth
	}

	// Create a timeout context for the send operation
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Try to send with timeout
	select {
	case bw.workerChannels[workerID] <- WriteRequest{Key: key, Value: value}:
		// Successfully sent to channel
		return nil
	case <-timeoutCtx.Done():
		// Record channel overflow if metrics are enabled
		if bw.metrics != nil {
			bw.metrics.RecordChannelOverflow(bw.ctx, workerID)
		}
		
		// Channel send timed out, try in background goroutine
		go func() {
			// Record non-blocking fallback if metrics are enabled
			if bw.metrics != nil {
				bw.metrics.RecordNonBlockingFallback(bw.ctx, workerID)
			}
			
			select {
			case bw.workerChannels[workerID] <- WriteRequest{Key: key, Value: value}:
				// Successfully sent
			case <-time.After(5 * time.Second):
				// If we can't send after a long timeout, log the error
				zap.L().Error("Failed to queue record for batch writing after extended timeout",
					zap.Binary("key_prefix", key[:8]))
			}
		}()
		return errors.New("queue operation timed out, write queued in background")
	}
}

// flush writes the buffered key-value pairs to the MDBX database in a single transaction for a given worker.
func (bw *BatchWriter) flush(workerID int) {
	if len(bw.workerBuffers[workerID]) == 0 {
		return
	}
	
	// Record metrics - store batch size and start time if metrics are enabled
	batchSize := len(bw.workerBuffers[workerID])
	var startTime time.Time
	if bw.metrics != nil {
		startTime = time.Now()
	}

	err := bw.db.env.Update(func(txn *mdbx.Txn) error {
		cursor, err := txn.OpenCursor(bw.db.GetDBI())
		if err != nil {
			return errors.Wrap(err, "failed to open cursor")
		}
		defer cursor.Close()

		// Write all buffered key-value pairs for this worker to the database
		for key, value := range bw.workerBuffers[workerID] {
			if err := cursor.Put(key[:], value, 0); err != nil {
				return errors.Wrapf(err, "failed to write key: %x", key)
			}
		}
		return nil
	})

	if err != nil {
		zap.L().Error(
			"failure to flush messages",
			zap.Error(err),
		)
		// Handle the error (logging or retry logic could be added here)
	}
	
	// Record metrics if enabled and the operation was successful
	if err == nil && bw.metrics != nil {
		flushDuration := time.Since(startTime)
		bw.metrics.RecordBatchProcessed(bw.ctx, workerID, batchSize, flushDuration)
	}

	// Clear the buffer after a successful flush
	bw.workerBuffers[workerID] = make(map[[32]byte][]byte)
	
	// Update worker buffer size metric if metrics are enabled
	if bw.metrics != nil {
		bw.metrics.UpdateWorkerBuffer(bw.ctx, workerID, 0)
	}
}

// FlushAndStop flushes any remaining data and stops the background workers.
func (bw *BatchWriter) FlushAndStop() {
	// Signal all workers to stop
	close(bw.stopChannel)
}

// Flush immediately flushes all pending writes across all workers.
// This is useful for testing when you need to ensure data is persisted.
func (bw *BatchWriter) Flush() {
	// Flush all workers
	for i := 0; i < bw.workers; i++ {
		bw.workerMutexes[i].Lock()
		bw.flush(i)
		bw.workerMutexes[i].Unlock()
	}
}
