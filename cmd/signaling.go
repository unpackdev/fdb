package cmd

import (
	"context"
	"fmt"
	"github.com/unpackdev/fdb/benchmark"
	"os"
	"os/signal"
	"syscall"
)

// handleBenchmarkSignals traps OS signals for graceful shutdown.
func handleBenchmarkSignals(ctx context.Context, suiteManager *benchmark.SuiteManager, suiteType benchmark.SuiteType) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		fmt.Println("Received interrupt signal, stopping suite...")
		suiteManager.Stop(ctx, suiteType)
		os.Exit(1)
	}()
}
