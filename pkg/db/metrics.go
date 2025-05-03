package db

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// BatchWriterMetrics holds all metrics instruments for the BatchWriter
type BatchWriterMetrics struct {
	// Counters
	BatchesProcessedTotal     metric.Int64Counter
	RecordsProcessedTotal     metric.Int64Counter
	ChannelOverflowsTotal     metric.Int64Counter
	NonBlockingFallbacksTotal metric.Int64Counter

	// Histograms
	BatchSizeHistogram     metric.Int64Histogram
	FlushDurationMs        metric.Float64Histogram
	WorkerQueueWaitTimeMs  metric.Float64Histogram

	// Gauges
	QueueDepth           metric.Int64UpDownCounter
	WorkerBufferSizes    []metric.Int64UpDownCounter
	ActiveWorkers        metric.Int64UpDownCounter
}

// InitializeBatchWriterMetrics creates and registers all BatchWriter metrics
func InitializeBatchWriterMetrics(ctx context.Context, meter metric.Meter) (*BatchWriterMetrics, error) {
	m := &BatchWriterMetrics{}
	var err error

	// Initialize counters
	m.BatchesProcessedTotal, err = meter.Int64Counter(
		"fdb_batch_writer_batches_processed_total",
		metric.WithDescription("Total number of batches processed by BatchWriter"),
	)
	if err != nil {
		return nil, err
	}

	m.RecordsProcessedTotal, err = meter.Int64Counter(
		"fdb_batch_writer_records_processed_total",
		metric.WithDescription("Total number of records processed by BatchWriter"),
	)
	if err != nil {
		return nil, err
	}

	m.ChannelOverflowsTotal, err = meter.Int64Counter(
		"fdb_batch_writer_channel_overflows_total",
		metric.WithDescription("Number of times worker channels reached capacity"),
	)
	if err != nil {
		return nil, err
	}

	m.NonBlockingFallbacksTotal, err = meter.Int64Counter(
		"fdb_batch_writer_non_blocking_fallbacks_total",
		metric.WithDescription("Number of times non-blocking fallback was used"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize histograms
	m.BatchSizeHistogram, err = meter.Int64Histogram(
		"fdb_batch_writer_batch_size",
		metric.WithDescription("Distribution of batch sizes processed by BatchWriter"),
	)
	if err != nil {
		return nil, err
	}

	m.FlushDurationMs, err = meter.Float64Histogram(
		"fdb_batch_writer_flush_duration_milliseconds",
		metric.WithDescription("Time taken to flush batches to disk in milliseconds"),
	)
	if err != nil {
		return nil, err
	}

	m.WorkerQueueWaitTimeMs, err = meter.Float64Histogram(
		"fdb_batch_writer_queue_wait_milliseconds",
		metric.WithDescription("Time requests spend waiting in worker queues in milliseconds"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize gauges
	m.QueueDepth, err = meter.Int64UpDownCounter(
		"fdb_batch_writer_queue_depth",
		metric.WithDescription("Current depth of all BatchWriter queues combined"),
	)
	if err != nil {
		return nil, err
	}

	m.ActiveWorkers, err = meter.Int64UpDownCounter(
		"fdb_batch_writer_active_workers",
		metric.WithDescription("Number of currently active worker goroutines"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordBatchProcessed records metrics for a processed batch
func (m *BatchWriterMetrics) RecordBatchProcessed(ctx context.Context, workerID int, batchSize int, duration time.Duration) {
	attrs := attribute.NewSet(attribute.Int("worker_id", workerID))
	
	m.BatchesProcessedTotal.Add(ctx, 1, metric.WithAttributeSet(attrs))
	m.RecordsProcessedTotal.Add(ctx, int64(batchSize), metric.WithAttributeSet(attrs))
	m.BatchSizeHistogram.Record(ctx, int64(batchSize), metric.WithAttributeSet(attrs))
	m.FlushDurationMs.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributeSet(attrs))
}

// RecordWorkerQueueWait records the time a request spends waiting in a worker queue
func (m *BatchWriterMetrics) RecordWorkerQueueWait(ctx context.Context, workerID int, duration time.Duration) {
	attrs := attribute.NewSet(attribute.Int("worker_id", workerID))
	m.WorkerQueueWaitTimeMs.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributeSet(attrs))
}

// RecordChannelOverflow records a channel overflow event
func (m *BatchWriterMetrics) RecordChannelOverflow(ctx context.Context, workerID int) {
	attrs := attribute.NewSet(attribute.Int("worker_id", workerID))
	m.ChannelOverflowsTotal.Add(ctx, 1, metric.WithAttributeSet(attrs))
}

// RecordNonBlockingFallback records when the non-blocking fallback is used
func (m *BatchWriterMetrics) RecordNonBlockingFallback(ctx context.Context, workerID int) {
	attrs := attribute.NewSet(attribute.Int("worker_id", workerID))
	m.NonBlockingFallbacksTotal.Add(ctx, 1, metric.WithAttributeSet(attrs))
}

// UpdateQueueDepth updates the queue depth counter
func (m *BatchWriterMetrics) UpdateQueueDepth(ctx context.Context, delta int64) {
	m.QueueDepth.Add(ctx, delta)
}

// UpdateWorkerBuffer updates the buffer size for a specific worker
func (m *BatchWriterMetrics) UpdateWorkerBuffer(ctx context.Context, workerID int, size int64) {
	if workerID < len(m.WorkerBufferSizes) {
		attrs := attribute.NewSet(attribute.Int("worker_id", workerID))
		m.WorkerBufferSizes[workerID].Add(ctx, size, metric.WithAttributeSet(attrs))
	}
}

// InitializeWorkerBufferMetrics creates separate buffer size metrics for each worker
func (m *BatchWriterMetrics) InitializeWorkerBufferMetrics(ctx context.Context, meter metric.Meter, workerCount int) error {
	m.WorkerBufferSizes = make([]metric.Int64UpDownCounter, workerCount)
	
	for i := 0; i < workerCount; i++ {
		var err error
		m.WorkerBufferSizes[i], err = meter.Int64UpDownCounter(
			"fdb_batch_writer_worker_buffer_size",
			metric.WithDescription("Current size of worker buffer"),
		)
		if err != nil {
			return err
		}
	}
	
	return nil
}
