package db

import (
	"github.com/erigontech/mdbx-go/mdbx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sync"
	"time"
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
}

// NewBatchWriter initializes a BatchWriter with a configurable number of workers.
func NewBatchWriter(db *Db, maxBatchSize int, flushInterval time.Duration, workers int) *BatchWriter {
	bw := &BatchWriter{
		db:             db,
		workerChannels: make([]chan WriteRequest, workers),
		workerBuffers:  make([]map[[32]byte][]byte, workers),
		workerMutexes:  make([]sync.Mutex, workers),
		maxBatchSize:   maxBatchSize,
		flushInterval:  flushInterval,
		stopChannel:    make(chan struct{}),
		workers:        workers,
	}

	// Initialize each worker's channel and buffer
	for i := 0; i < workers; i++ {
		bw.workerChannels[i] = make(chan WriteRequest, 500000) // Dedicated buffered channel for each worker
		bw.workerBuffers[i] = make(map[[32]byte][]byte)
		go bw.runWorker(i)
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
			bw.workerMutexes[workerID].Lock()
			// Add the request to the worker's buffer
			bw.workerBuffers[workerID][req.Key] = req.Value

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
func (bw *BatchWriter) BufferWrite(key [32]byte, value []byte) {
	// Determine which worker to assign the write to (for simplicity, we can use modulo)
	workerID := int(key[0]) % bw.workers
	bw.workerChannels[workerID] <- WriteRequest{Key: key, Value: value}
}

// flush writes the buffered key-value pairs to the MDBX database in a single transaction for a given worker.
func (bw *BatchWriter) flush(workerID int) {
	if len(bw.workerBuffers[workerID]) == 0 {
		return
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

	// Clear the buffer after a successful flush
	bw.workerBuffers[workerID] = make(map[[32]byte][]byte)
}

// FlushAndStop flushes any remaining data and stops the background workers.
func (bw *BatchWriter) FlushAndStop() {
	// Signal all workers to stop
	close(bw.stopChannel)
}
