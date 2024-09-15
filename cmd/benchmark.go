package cmd

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"log"
	"runtime"
	"time"
)

// BenchmarkCommand returns a cli.Command that benchmarks the real client
func BenchmarkCommand() *cli.Command {
	return &cli.Command{
		Name:  "test",
		Usage: "Benchmark the real client",
		Action: func(c *cli.Context) error {
			// Simulate benchmarking logic here
			fmt.Println("Running client benchmark...")

			// Example: Perform a benchmark by running some mock client operations
			benchmarkClient()

			return nil
		},
	}
}

// benchmarkClient simulates client benchmarking
func benchmarkClient() {
	// Capture initial memory usage
	var memStart runtime.MemStats
	runtime.ReadMemStats(&memStart)

	// Simulate some work being done by the client
	start := time.Now()

	// Replace this loop with real benchmarking code
	for i := 0; i < 1000; i++ {
		// Simulate work by sleeping
		time.Sleep(1 * time.Millisecond)
	}

	// Measure memory usage after
	var memEnd runtime.MemStats
	runtime.ReadMemStats(&memEnd)

	elapsed := time.Since(start)

	// Output benchmarking results
	fmt.Printf("Benchmark completed in %s\n", elapsed)
	fmt.Printf("Memory used: %d bytes\n", memEnd.Alloc-memStart.Alloc)
	log.Printf("Test completed.")
}
