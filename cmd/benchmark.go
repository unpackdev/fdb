package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/benchmark"
	"github.com/unpackdev/fdb/config"
	"github.com/urfave/cli/v2"
	"time"
)

// BenchmarkCommand returns a cli.Command that benchmarks the real client.
func BenchmarkCommand() *cli.Command {
	return &cli.Command{
		Name:  "benchmark",
		Usage: "Run benchmark tests on different transport protocols",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Usage: "Path where benchmark configuration can be found",
				Value: "./benchmark.yaml",
			},
			&cli.StringFlag{
				Name:  "suite",
				Usage: "Specify the suite type (e.g., quic, dummy)",
				Value: "dummy", // Default to DUMMY
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: "Specify the benchmark type (e.g., write, read)",
				Value: "write", // Default to write benchmark
			},
			&cli.IntFlag{
				Name:  "clients",
				Usage: "Number of clients to simulate",
				Value: 10, // Default to 10 clients
			},
			&cli.IntFlag{
				Name:  "messages",
				Usage: "Number of messages per client",
				Value: 100000, // Default to 100000 messages per client
			},
			&cli.StringFlag{
				Name:  "output",
				Usage: "Path to save the JSON report (optional)",
				Value: "", // Default to no export
			},
			&cli.IntFlag{
				Name:  "timeout",
				Usage: "Specify the timeout for the benchmark in seconds",
				Value: 60, // Default to 60 seconds timeout
			},
		},
		Action: func(c *cli.Context) error {
			// Load the config.yaml file
			configPath := c.String("config")
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return errors.Wrap(err, "failed to load configuration")
			}

			// Initialize FDB
			fdbc, err := fdb.New(c.Context, *cfg)
			if err != nil {
				return fmt.Errorf("failed to initialize FDB: %w", err)
			}

			// Create a new suite manager
			suiteManager := benchmark.NewSuiteManager(fdbc)

			// Set up signal handling for graceful shutdown
			handleBenchmarkSignals(c.Context, suiteManager, benchmark.SuiteType(c.String("suite-type")))

			// Get the suite type, benchmark type, and number of clients/messages from CLI flags
			suiteType := benchmark.SuiteType(c.String("suite"))
			benchmarkType := c.String("type")
			totalClients := c.Int("clients")
			messagesPerClient := c.Int("messages")
			timeout := time.Duration(c.Int("timeout")) * time.Second

			// Start the suite
			if err := suiteManager.Start(c.Context, suiteType); err != nil {
				return fmt.Errorf("failed to start suite: %w", err)
			}

			defer suiteManager.Stop(c.Context, suiteType)

			report := benchmark.NewReport()

			// Create a context with a timeout
			ctx, cancel := context.WithTimeout(c.Context, timeout)
			defer cancel()

			// Run the appropriate benchmark based on the user's choice (write or read)
			switch benchmarkType {
			case "write":
				if rErr := suiteManager.RunWriteBenchmark(ctx, suiteType, totalClients, messagesPerClient, report); rErr != nil {
					return errors.Wrap(rErr, "failed to run write benchmark")
				}
			case "read":
				if rErr := suiteManager.RunReadBenchmark(ctx, suiteType, totalClients, messagesPerClient, report); rErr != nil {
					return errors.Wrap(rErr, "failed to run read benchmark")
				}

			default:
				return fmt.Errorf("invalid benchmark type: %s", benchmarkType)
			}

			// Print report to console
			report.PrintReport()

			// Optional: export report to JSON if the report-output flag is set
			if outputPath := c.String("output"); outputPath != "" {
				if err := report.ExportToJSON(outputPath); err != nil {
					return fmt.Errorf("failed to export report: %w", err)
				}
			}

			return nil
		},
	}
}
