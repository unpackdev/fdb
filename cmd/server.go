package cmd

import (
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/config"
	"github.com/urfave/cli/v2"
)

// ServerCommand returns a cli.Command that benchmarks the real client
func ServerCommand() *cli.Command {
	return &cli.Command{
		Name:  "start",
		Usage: "Start (f)db transport server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Usage: "Path where benchmark configuration can be found",
				Value: "./benchmark.yaml",
			},
			&cli.StringSliceFlag{
				Name:  "transports",
				Usage: "Specify the transport types (e.g., quic, uds)",
				Value: cli.NewStringSlice("quic", "uds"), // Corrected default initialization
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
				return errors.Wrap(err, "failed to initialize FDB")
			}

			_ = fdbc
			return nil
		},
	}
}
