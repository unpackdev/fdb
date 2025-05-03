package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/unpackdev/fdb/client"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/messages"
	"github.com/unpackdev/fdb/pkg/types"
)

// BenchmarkConfig holds configuration for a benchmark run
type BenchmarkConfig struct {
	Connections    int               // Number of parallel connections
	MessageSize    int               // Size of each message in bytes
	MessageCount   int               // Total number of messages to send
	BatchSize      int               // Number of messages to batch in one send
	Interval       time.Duration     // Interval between sends
	WaitResponse   bool              // Whether to wait for response
	Timeout        time.Duration     // Response timeout
	HandlerType    types.HandlerType // Message handler type
	MaxConcurrency int               // Maximum concurrent operations
}

// BenchmarkResult holds results for a single message send
type BenchmarkResult struct {
	MessageNumber int
	MessageSize   int
	ResponseSize  int
	Elapsed       time.Duration
	Error         error
	ConnectionID  int
	Timestamp     time.Time
}

// BenchmarkSummary holds aggregated benchmark results
type BenchmarkSummary struct {
	TotalMessages      int
	SuccessfulMessages int
	FailedMessages     int
	TotalBytes         int64
	ResponseBytes      int64
	TotalDuration      time.Duration
	AverageLatency     time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	MedianLatency      time.Duration
	P95Latency         time.Duration
	P99Latency         time.Duration
	MBps               float64
	MsgPerSecond       float64
	ConnectionResults  map[int]int    // messages per connection
	Errors             map[string]int // error types and counts
	StartTime          time.Time
	EndTime            time.Time
	Config             BenchmarkConfig
}

// percentile calculates the specified percentile from a sorted slice of durations
func percentile(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	if len(durations) == 1 {
		return durations[0]
	}

	index := int(math.Ceil(float64(len(durations))*p/100.0)) - 1
	if index < 0 {
		index = 0
	}

	return durations[index]
}

// RunBenchmark executes a benchmark with the given configuration
func RunBenchmark(ctx context.Context, c *client.Client, cfg BenchmarkConfig, log logger.Logger) (*BenchmarkSummary, error) {
	log.Info("Starting benchmark",
		"connections", cfg.Connections,
		"message_size", cfg.MessageSize,
		"message_count", cfg.MessageCount,
		"wait_response", cfg.WaitResponse,
		"interval", cfg.Interval)

	// Channel for collecting results
	resultCh := make(chan BenchmarkResult, cfg.MessageCount)

	// Set a reasonable default timeout if none is specified
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	// Create a progress reporting goroutine
	progressCtx, cancelProgress := context.WithCancel(ctx)
	progressInterval := 2 * time.Second
	go reportProgress(progressCtx, resultCh, cfg.MessageCount, progressInterval, log)

	// Semaphore for limiting concurrency
	var semaphore chan struct{}
	if cfg.MaxConcurrency > 0 {
		semaphore = make(chan struct{}, cfg.MaxConcurrency)
	}

	// Create connections
	var wg sync.WaitGroup
	startTime := time.Now()

	// Divide messages among connections
	messagesPerConnection := cfg.MessageCount / cfg.Connections
	if messagesPerConnection == 0 {
		messagesPerConnection = 1
	}

	// Start workers
	for connID := 0; connID < cfg.Connections; connID++ {
		wg.Add(1)
		go func(connectionID int) {
			defer wg.Done()

			// Calculate message range for this connection
			startMsg := connectionID * messagesPerConnection
			endMsg := startMsg + messagesPerConnection
			if connectionID == cfg.Connections-1 {
				endMsg = cfg.MessageCount // Last connection handles remaining messages
			}

			// Get transport
			transport, err := c.GetTransport("tcp")
			if err != nil {
				resultCh <- BenchmarkResult{
					Error:        fmt.Errorf("failed to get transport: %w", err),
					ConnectionID: connectionID,
					Timestamp:    time.Now(),
				}
				return
			}

			tcpTransport, ok := transport.(*client.TCPTransport)
			if !ok {
				resultCh <- BenchmarkResult{
					Error:        fmt.Errorf("transport is not a TCPTransport"),
					ConnectionID: connectionID,
					Timestamp:    time.Now(),
				}
				return
			}

			// Process messages
			for msgNum := startMsg; msgNum < endMsg; msgNum++ {
				// Use semaphore if concurrency limiting is enabled
				if semaphore != nil {
					semaphore <- struct{}{}
				}

				// Wait for interval between messages
				if cfg.Interval > 0 && msgNum > startMsg {
					time.Sleep(cfg.Interval)
				}

				// Create message data
				data := generateBenchmarkData(cfg.MessageSize, msgNum)

				// Generate a message with the data
				msg, err := messages.GenerateRandomMessageWithData(cfg.HandlerType, data)
				if err != nil {
					resultCh <- BenchmarkResult{
						MessageNumber: msgNum,
						MessageSize:   cfg.MessageSize,
						Error:         err,
						ConnectionID:  connectionID,
						Timestamp:     time.Now(),
					}
					if semaphore != nil {
						<-semaphore
					}
					continue
				}

				encodedMsg, err := msg.Encode()
				if err != nil {
					resultCh <- BenchmarkResult{
						MessageNumber: msgNum,
						MessageSize:   cfg.MessageSize,
						Error:         err,
						ConnectionID:  connectionID,
						Timestamp:     time.Now(),
					}
					if semaphore != nil {
						<-semaphore
					}
					continue
				}

				// For large payloads, use chunking protocol
				if len(encodedMsg) > maxChunkSize {
					// Prepare the chunked message with a 4-byte length prefix
					lengthPrefix := make([]byte, 4)
					binary.LittleEndian.PutUint32(lengthPrefix, uint32(len(encodedMsg)))

					// Prepend the length prefix to the message
					chunkedMsg := append(lengthPrefix, encodedMsg...)

					// Replace the original message with the chunked version
					encodedMsg = chunkedMsg
				}

				// Send and measure
				startMsg := time.Now()
				var responseSize int

				if cfg.WaitResponse {
					// Register response channel
					responseType := client.MessageType(types.HandlerStatusSuccess.Byte())
					responseCh := tcpTransport.RegisterResponseChannel(responseType)

					// Send the message
					if err := tcpTransport.Send(encodedMsg); err != nil {
						tcpTransport.UnregisterResponseChannel(responseType)
						resultCh <- BenchmarkResult{
							MessageNumber: msgNum,
							MessageSize:   cfg.MessageSize,
							Error:         err,
							Elapsed:       time.Since(startMsg),
							ConnectionID:  connectionID,
							Timestamp:     time.Now(),
						}
						if semaphore != nil {
							<-semaphore
						}
						continue
					}

					// Wait for response with a shorter timeout per message
					// This is particularly important for high throughput tests
					responseTimeout := cfg.Timeout
					if cfg.Interval == 0 {
						// Use a much shorter timeout for no-interval benchmarks to prevent hanging
						responseTimeout = 500 * time.Millisecond
					}

					response, err := tcpTransport.WaitForResponseWithTimeout(responseCh, responseTimeout)
					if err != nil {
						resultCh <- BenchmarkResult{
							MessageNumber: msgNum,
							MessageSize:   cfg.MessageSize,
							Error:         err,
							Elapsed:       time.Since(startMsg),
							ConnectionID:  connectionID,
							Timestamp:     time.Now(),
						}
						if semaphore != nil {
							<-semaphore
						}
						continue
					}

					responseSize = len(response)
				} else {
					// Send without waiting for response
					if err := tcpTransport.Send(encodedMsg); err != nil {
						resultCh <- BenchmarkResult{
							MessageNumber: msgNum,
							MessageSize:   cfg.MessageSize,
							Error:         err,
							Elapsed:       time.Since(startMsg),
							ConnectionID:  connectionID,
							Timestamp:     time.Now(),
						}
						if semaphore != nil {
							<-semaphore
						}
						continue
					}
				}

				elapsedTime := time.Since(startMsg)

				// Record successful result
				resultCh <- BenchmarkResult{
					MessageNumber: msgNum,
					MessageSize:   cfg.MessageSize,
					ResponseSize:  responseSize,
					Elapsed:       elapsedTime,
					ConnectionID:  connectionID,
					Timestamp:     time.Now(),
				}

				// Release semaphore
				if semaphore != nil {
					<-semaphore
				}
			}
		}(connID)
	}

	// Close the result channel when all workers complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results with a hard timeout to prevent hanging
	results := make([]BenchmarkResult, 0, cfg.MessageCount)
	timeoutTimer := time.NewTimer(time.Duration(cfg.MessageCount)*cfg.Timeout + 10*time.Second)
	messagesReceived := 0

	for messagesReceived < cfg.MessageCount {
		select {
		case result := <-resultCh:
			results = append(results, result)
			messagesReceived++
		case <-timeoutTimer.C:
			log.Warn("Benchmark collection timed out",
				"expected", cfg.MessageCount,
				"received", messagesReceived)
			goto processResults
		case <-ctx.Done():
			log.Warn("Benchmark cancelled",
				"expected", cfg.MessageCount,
				"received", messagesReceived)
			goto processResults
		}
	}

processResults:
	// Stop the progress reporting
	cancelProgress()

	endTime := time.Now()

	// Process results into summary
	summary := processBenchmarkResults(results, startTime, endTime, cfg)

	// Print detailed report
	PrintBenchmarkReport(summary, log)

	return summary, nil
}

func reportProgress(ctx context.Context, resultCh chan BenchmarkResult, totalMessages int, interval time.Duration, log logger.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Keep a local count to avoid reading from the channel
	var messagesProcessed int

	for {
		select {
		case <-ticker.C:
			// Calculate approximate count from channel capacity
			approxProcessed := totalMessages - len(resultCh)
			if approxProcessed > messagesProcessed {
				messagesProcessed = approxProcessed
			}

			percentComplete := float64(messagesProcessed) / float64(totalMessages) * 100.0
			log.Info("Benchmark progress",
				"processed", messagesProcessed,
				"total", totalMessages,
				"percent", fmt.Sprintf("%.1f%%", percentComplete))
		case <-ctx.Done():
			return
		}
	}
}

// generateBenchmarkData creates data for benchmarking
func generateBenchmarkData(size, seed int) []byte {
	data := make([]byte, size)

	// Add a timestamp and message number at the start of the data for tracing
	timestamp := time.Now().UnixNano()
	if size >= 12 {
		// Add timestamp (8 bytes) and message number (4 bytes) at the start
		binary.LittleEndian.PutUint64(data[0:8], uint64(timestamp))
		binary.LittleEndian.PutUint32(data[8:12], uint32(seed))

		// Fill the rest with a pattern
		for i := 12; i < size; i++ {
			data[i] = byte((i + seed) % 256)
		}
	} else {
		// If the buffer is too small, just fill it with a pattern
		for i := range data {
			data[i] = byte((i + seed) % 256)
		}
	}

	return data
}

// processBenchmarkResults aggregates individual results into a summary
func processBenchmarkResults(results []BenchmarkResult, startTime, endTime time.Time, cfg BenchmarkConfig) *BenchmarkSummary {
	summary := &BenchmarkSummary{
		TotalMessages:     len(results),
		ConnectionResults: make(map[int]int),
		Errors:            make(map[string]int),
		StartTime:         startTime,
		EndTime:           endTime,
		Config:            cfg,
		MinLatency:        time.Hour, // Start with a large value
	}

	// Collect success/failure stats
	var latencies []time.Duration
	for _, result := range results {
		if result.Error != nil {
			summary.FailedMessages++
			errorMsg := result.Error.Error()
			summary.Errors[errorMsg]++
		} else {
			summary.SuccessfulMessages++
			summary.TotalBytes += int64(result.MessageSize)
			summary.ResponseBytes += int64(result.ResponseSize)
			summary.TotalDuration += result.Elapsed

			// Track latency stats
			latencies = append(latencies, result.Elapsed)
			if result.Elapsed < summary.MinLatency {
				summary.MinLatency = result.Elapsed
			}
			if result.Elapsed > summary.MaxLatency {
				summary.MaxLatency = result.Elapsed
			}
		}

		summary.ConnectionResults[result.ConnectionID]++
	}

	// Calculate throughput
	totalDuration := endTime.Sub(startTime)
	if totalDuration > 0 {
		summary.MBps = float64(summary.TotalBytes) / (1024 * 1024) / totalDuration.Seconds()
		summary.MsgPerSecond = float64(summary.SuccessfulMessages) / totalDuration.Seconds()
	}

	// Calculate latency percentiles
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		summary.MedianLatency = percentile(latencies, 50)
		summary.P95Latency = percentile(latencies, 95)
		summary.P99Latency = percentile(latencies, 99)

		if summary.SuccessfulMessages > 0 {
			summary.AverageLatency = summary.TotalDuration / time.Duration(summary.SuccessfulMessages)
		}
	}

	return summary
}

// PrintBenchmarkReport prints a detailed benchmark report
func PrintBenchmarkReport(summary *BenchmarkSummary, log logger.Logger) {
	fmt.Println("\n========== BENCHMARK REPORT ==========")
	fmt.Printf("Duration: %v\n", summary.EndTime.Sub(summary.StartTime))
	fmt.Printf("Messages: %d total, %d successful, %d failed\n",
		summary.TotalMessages, summary.SuccessfulMessages, summary.FailedMessages)

	fmt.Printf("\nThroughput:\n")
	fmt.Printf("  %.2f MB/s\n", summary.MBps)
	fmt.Printf("  %.2f messages/s\n", summary.MsgPerSecond)

	if summary.SuccessfulMessages > 0 {
		fmt.Printf("\nLatency:\n")
		fmt.Printf("  Min: %v\n", summary.MinLatency)
		fmt.Printf("  Avg: %v\n", summary.AverageLatency)
		fmt.Printf("  Med: %v\n", summary.MedianLatency)
		fmt.Printf("  P95: %v\n", summary.P95Latency)
		fmt.Printf("  P99: %v\n", summary.P99Latency)
		fmt.Printf("  Max: %v\n", summary.MaxLatency)
	}

	if len(summary.ConnectionResults) > 0 {
		fmt.Printf("\nConnection Distribution:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Connection ID\tMessages\t%%\n")
		for connID, count := range summary.ConnectionResults {
			percentage := float64(count) / float64(summary.TotalMessages) * 100
			fmt.Fprintf(w, "  %d\t%d\t%.1f%%\n", connID, count, percentage)
		}
		w.Flush()
	}

	if len(summary.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Error\tCount\t%%\n")
		for errMsg, count := range summary.Errors {
			percentage := float64(count) / float64(summary.FailedMessages) * 100
			fmt.Fprintf(w, "  %s\t%d\t%.1f%%\n", errMsg, count, percentage)
		}
		w.Flush()
	}

	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Connections: %d\n", summary.Config.Connections)
	fmt.Printf("  Message Size: %d bytes\n", summary.Config.MessageSize)
	fmt.Printf("  Message Count: %d\n", summary.Config.MessageCount)
	fmt.Printf("  Wait For Response: %v\n", summary.Config.WaitResponse)
	fmt.Printf("  Handler Type: %s\n", summary.Config.HandlerType.String())
	if summary.Config.MaxConcurrency > 0 {
		fmt.Printf("  Max Concurrency: %d\n", summary.Config.MaxConcurrency)
	}

	fmt.Println("======================================")
}
