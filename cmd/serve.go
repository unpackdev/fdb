package cmd

import (
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/types"
	"github.com/urfave/cli/v2"
)

// ServeCommand returns a cli.Command that benchmarks the real client
func ServeCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start (f)db transport server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Usage: "Path where benchmark configuration can be found",
				Value: "./config.yaml",
			},
			&cli.StringSliceFlag{
				Name:  "transports",
				Usage: "Specify the transport types (e.g., quic, uds)",
				Value: cli.NewStringSlice("quic", "uds"), // Corrected default initialization
			},
		},
		Action: func(c *cli.Context) error {
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

			transports := make([]types.TransportType, 0)
			for _, t := range c.StringSlice("transports") {
				tt, ttErr := types.ParseTransportType(t)
				if ttErr != nil {
					return errors.Wrapf(ttErr, "invalid transport type: %s", t)
				}

				transports = append(transports, tt)
			}
			return fdbc.Start(c.Context, transports...)
		},
	}
}
