package fdb

/*import (
	"context"
	"log"
	"sync"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var serverStarted sync.WaitGroup
	serverStarted.Add(1)

	// Create a new UDP server instance
	server, err := New(8781, "127.0.0.1")
	if err != nil {
		log.Fatalf("Failed to create UDP server: %v", err)
	}

	// Register handlers
	server.RegisterHandler(WriteHandlerType, WriteHandler)
	server.RegisterHandler(ReadHandlerType, ReadHandler)

	// Start the server in a separate goroutine
	go func() {
		defer serverStarted.Done()
		server.Start()
	}()

	// Wait for the server to start
	serverStarted.Wait()

	// Simulate running for a while before shutdown
	select {
	case <-ctx.Done():
		log.Println("Server shutting down.")
		server.Stop() // Gracefully stop the server
	}
}*/
