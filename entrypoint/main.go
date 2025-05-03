package main

import (
	"log"
	"os"

	"github.com/unpackdev/fdb/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "(f)db",
		Usage: "Fast Database Transports",
		Commands: []*cli.Command{
			cmd.CertsCommand(),      // Command for handling certificates
			cmd.BenchmarkCommand(),  // Command for running benchmarks
			cmd.EbpfCommands(),      // Command for running eBPF specific workload
			cmd.ServeCommand(),      // Command to start the server
			cmd.KeystoreCommand(),   // Command for handling keystore (Peer IDs)
			cmd.PlaygroundCommand(), // Command for playing with real clients over simulated network
		},
	}

	// Run the app and handle any errors
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Error running CLI: %v", err)
	}
}
