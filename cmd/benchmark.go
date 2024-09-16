package cmd

import (
	"fmt"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/benchmark"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/types"
	"github.com/urfave/cli/v2"
)

// BenchmarkCommand returns a cli.Command that benchmarks the real client.
func BenchmarkCommand() *cli.Command {
	return &cli.Command{
		Name:  "benchmark",
		Usage: "Run benchmark tests on different transport protocols",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "suite-type",
				Usage: "Specify the suite type (e.g., quic)",
				Value: "quic", // Default to QUIC
			},
			&cli.IntFlag{
				Name:  "clients",
				Usage: "Number of clients to simulate",
				Value: 10, // Default to 10 clients
			},
			&cli.IntFlag{
				Name:  "messages",
				Usage: "Number of messages per client",
				Value: 100, // Default to 100 messages per client
			},
			&cli.StringFlag{
				Name:  "report-output",
				Usage: "Path to save the JSON report (optional)",
				Value: "", // Default to no export
			},
		},
		Action: func(c *cli.Context) error {
			fmt.Println("Starting benchmark...")

			// Configure transports (for now just QUIC)
			cnf := config.Config{
				Logger: config.Logger{
					Enabled:     true,
					Environment: "development",
					Level:       "info",
				},
				MdbxNodes: []config.MdbxNode{
					{
						Name: "benchmark",
						Path: "/tmp/",
					},
				},
				Transports: []config.Transport{
					{
						Type:    types.QUICTransportType,
						Enabled: true,
						Config: config.QuicTransport{
							Enabled: true,
							IPv4:    "127.0.0.1",
							Port:    4433,
							TLS: config.TLS{
								Key:    "./data/certs/key.pem",
								Cert:   "./data/certs/cert.pem",
								RootCA: "",
							},
						},
					},
					{
						Type:    types.DummyTransportType,
						Enabled: true,
						Config: config.DummyTransport{
							Enabled: true,
							IPv4:    "127.0.0.1",
							Port:    4434,
							TLS: config.TLS{
								Key:    "./data/certs/key.pem",
								Cert:   "./data/certs/cert.pem",
								RootCA: "",
							},
						},
					},
				},
			}

			// Initialize FDB
			fdbc, err := fdb.New(c.Context, cnf)
			if err != nil {
				return fmt.Errorf("failed to initialize FDB: %w", err)
			}

			// Create a new suite manager
			suiteManager := benchmark.NewSuiteManager(fdbc)

			// Get the suite type and number of clients/messages from CLI flags
			suiteType := benchmark.SuiteType(c.String("suite-type"))
			totalClients := c.Int("clients")
			messagesPerClient := c.Int("messages")

			// Start the suite
			if err := suiteManager.Start(c.Context, suiteType); err != nil {
				return fmt.Errorf("failed to start suite: %w", err)
			}

			defer suiteManager.Stop(c.Context, suiteType)

			report := benchmark.NewReport()

			// Create client pool and start the benchmarking process
			clientPool := benchmark.NewClientPool(totalClients, messagesPerClient, report)
			err = clientPool.Start(c.Context, suiteManager.Suites[suiteType])
			if err != nil {
				return fmt.Errorf("failed to run client pool: %w", err)
			}

			// Finalize the results (latency, throughput)
			clientPool.Finalize()

			// Print report to console
			report.PrintReport()

			// Optionally export report to JSON
			reportOutput := c.String("report-output")
			if reportOutput != "" {
				if err := report.ExportToJSON(reportOutput); err != nil {
					return fmt.Errorf("failed to export report: %w", err)
				}
			}

			return nil
		},
	}
}
