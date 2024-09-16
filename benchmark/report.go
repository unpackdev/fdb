package benchmark

import (
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	"os"
	"time"
)

// Report holds the results of the benchmark.
type Report struct {
	TotalClients     int             `json:"total_clients"`
	TotalMessages    int             `json:"total_messages"`
	SuccessMessages  int             `json:"success_messages"`
	FailedMessages   int             `json:"failed_messages"`
	TotalDuration    time.Duration   `json:"total_duration"`
	AvgLatency       time.Duration   `json:"avg_latency"`
	Throughput       float64         `json:"throughput"` // Messages per second
	MemoryUsed       uint64          `json:"memory_used"`
	LatencyHistogram []time.Duration `json:"latency_histogram"`
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
	fmt.Printf("Total Messages: %d\n", r.TotalMessages)
	fmt.Printf("Success Messages: %d\n", r.SuccessMessages)
	fmt.Printf("Failed Messages: %d\n", r.FailedMessages)
	fmt.Printf("Total Duration: %s\n", r.TotalDuration)
	fmt.Printf("Average Latency: %s\n", r.AvgLatency)
	fmt.Printf("Throughput: %s messages/second\n", humanize.Comma(int64(r.Throughput)))
	fmt.Printf("Memory Used: %d bytes\n", r.MemoryUsed)
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
