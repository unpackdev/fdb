package benchmark

import (
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	"os"
	"sort"
	"time"
)

// Report holds the results of the benchmark.
type Report struct {
	TotalClients      int             `json:"total_clients"`
	MessagesPerClient int             `json:"messages_per_client"`
	TotalMessages     int             `json:"total_messages"`
	SuccessMessages   int             `json:"success_messages"`
	FailedMessages    int             `json:"failed_messages"`
	TotalDuration     time.Duration   `json:"total_duration"`
	AvgLatency        time.Duration   `json:"avg_latency"`
	Throughput        float64         `json:"throughput"` // Messages per second
	MemoryUsed        uint64          `json:"memory_used"`
	LatencyHistogram  []time.Duration `json:"latency_histogram"`
	Jitter            float64         `json:"jitter"`          // Latency jitter (standard deviation)
	LatencyStdDev     float64         `json:"latency_std_dev"` // Standard deviation of latencies
	P50Latency        time.Duration   `json:"p50_latency"`     // 50th percentile (median)
	P90Latency        time.Duration   `json:"p90_latency"`     // 90th percentile
	P99Latency        time.Duration   `json:"p99_latency"`     // 99th percentile
}

// NewReport creates a new benchmark report.
func NewReport() *Report {
	return &Report{
		LatencyHistogram: make([]time.Duration, 0),
	}
}

// PrintReport prints the benchmark report to the console.
func (r *Report) PrintReport() {
	fmt.Printf("\n--- Benchmark Report ---\n")
	fmt.Printf("Total Clients: %d\n", r.TotalClients)
	fmt.Printf("Messages per Client: %d\n", r.MessagesPerClient)
	fmt.Printf("Total Messages: %d\n", r.TotalMessages)
	fmt.Printf("Success Messages: %d\n", r.SuccessMessages)
	fmt.Printf("Failed Messages: %d\n", r.FailedMessages)
	fmt.Printf("Total Duration: %s\n", r.TotalDuration)
	fmt.Printf("Average Latency: %s\n", r.AvgLatency)
	fmt.Printf("P50 Latency: %s\n", r.P50Latency)
	fmt.Printf("P90 Latency: %s\n", r.P90Latency)
	fmt.Printf("P99 Latency: %s\n", r.P99Latency)
	fmt.Printf("Throughput: %s messages/second\n", humanize.Comma(int64(r.Throughput)))
	// Convert memory used from bytes to MB and print
	memoryUsedMB := float64(r.MemoryUsed) / (1024 * 1024)
	fmt.Printf("Memory Used: %.2f MB\n", memoryUsedMB)
	fmt.Printf("Latency Jitter (StdDev): %.6fµs\n", r.Jitter*1e6) // Print jitter in microseconds
	fmt.Println("")
}

// Finalize aggregates the latency data and updates the benchmark report.
func (r *Report) Finalize() {
	// Calculate total messages as the product of clients and messages per client
	r.TotalMessages = r.TotalClients * r.MessagesPerClient

	// Calculate average latency if there are successful messages
	var totalLatency time.Duration
	for _, latency := range r.LatencyHistogram {
		totalLatency += latency
	}

	if len(r.LatencyHistogram) > 0 {
		// Calculate average latency
		r.AvgLatency = totalLatency / time.Duration(len(r.LatencyHistogram))

		// Sort the histogram to calculate percentiles
		sort.Slice(r.LatencyHistogram, func(i, j int) bool {
			return r.LatencyHistogram[i] < r.LatencyHistogram[j]
		})

		// Calculate percentiles
		r.P50Latency = r.LatencyHistogram[len(r.LatencyHistogram)/2]
		r.P90Latency = r.LatencyHistogram[int(float64(len(r.LatencyHistogram))*0.9)]
		r.P99Latency = r.LatencyHistogram[int(float64(len(r.LatencyHistogram))*0.99)]
	}

	// Calculate throughput based on the success messages and total duration
	if r.TotalDuration.Seconds() > 0 {
		r.Throughput = float64(r.SuccessMessages) / r.TotalDuration.Seconds()
	} else {
		r.Throughput = 0
	}

	// Capture memory usage
	r.MemoryUsed = GetMemoryUsage()

	// Calculate jitter (standard deviation of latencies)
	r.Jitter = calculateStdDev(r.LatencyHistogram)
}

// ExportToJSON exports the benchmark report to a JSON file.
func (r *Report) ExportToJSON(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	err = encoder.Encode(r)
	if err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	fmt.Printf("Benchmark report exported to %s\n", filename)
	return nil
}
