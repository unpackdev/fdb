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
		Usage: "Benchmark the real client",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "suite-type",
				Usage: "Specify the suite type (e.g., quic)",
				Value: "quic", // Default to quic
			},
		},
		Action: func(c *cli.Context) error {
			fmt.Println("Running client benchmark...")

			// Configure QUIC transport
			cnf := config.Config{
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
				},
			}

			// Initialize FDB
			fdbc, err := fdb.New(c.Context, cnf)
			if err != nil {
				return err
			}

			// Create a new suite manager
			suiteManager := benchmark.NewSuiteManager(fdbc)

			// Get the suite type from CLI flag
			suiteType := benchmark.SuiteType(c.String("suite-type"))

			// Start the suite
			if err := suiteManager.Start(suiteType); err != nil {
				return fmt.Errorf("failed to start suite: %w", err)
			}
			defer suiteManager.Stop(suiteType)

			// Run client-side benchmark
			if err := suiteManager.Run(c.Context, suiteType); err != nil {
				return fmt.Errorf("failed to run client benchmark: %w", err)
			}

			return nil
		},
	}
}
