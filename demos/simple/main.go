package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/unpackdev/fdb/client"
	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/messages"
	"github.com/unpackdev/fdb/pkg/types"
)

const (
	// maxChunkSize keeps every TCP write comfortably below the default Ethernet
	// MTU once TCP/IP framing overhead is added.
	maxChunkSize = 60 * 1024 // 60 KiB
)

var (
	// Connection parameters
	hostAddr   = flag.String("host", "127.0.0.1", "Host address of the FDB node")
	port       = flag.Int("port", 5011, "Port of the FDB node")
	bufferSize = flag.Int("buffer", 256, "Socket buffer size in KB")
	multicore  = flag.Bool("multicore", true, "Enable multicore processing")
	tcpNoDelay = flag.Bool("nodelay", true, "Enable TCP_NODELAY")

	// Message parameters
	sendMsg     = flag.Bool("send", false, "Send a test message after connecting")
	msgSize     = flag.Int("size", 1024, "Size of test message in bytes")
	msgCount    = flag.Int("count", 1, "Number of messages to send")
	msgInterval = flag.Duration("interval", 1*time.Second, "Interval between messages")
	msgType     = flag.String("type", "write", "Message type: write, read")

	// Response handling
	waitResponse = flag.Bool("response", false, "Wait for response after sending")
	timeout      = flag.Duration("timeout", 5*time.Second, "Response timeout")

	// Benchmark parameters
	benchmark     = flag.Bool("benchmark", false, "Run comprehensive benchmark")
	connections   = flag.Int("connections", 1, "Number of parallel connections for benchmark")
	maxConcurrent = flag.Int("concurrency", 0, "Maximum concurrent operations (0 = unlimited)")
	batchSize     = flag.Int("batch", 1, "Number of messages to batch in one send")
)

func main() {
	flag.Parse()

	// Set up logging
	// Use the global logger with default configuration
	loggerConfig := config.Logger{
		Enabled:     true,
		Level:       "info",
		Environment: "development",
	}
	log, err := logger.InitializeGlobalLogger(loggerConfig)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	log.Info("Starting FDB Simple Client")

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize client
	clientCfg := client.NewConfig()
	fdbClient := client.NewClient(ctx, clientCfg)

	// Build the target address
	targetAddr := fmt.Sprintf("%s:%d", *hostAddr, *port)
	log.Info("Connecting to FDB node", "address", targetAddr)

	// Create a TCP transport with optimized settings for performance
	var opts []gnet.Option
	if *multicore {
		opts = append(opts, gnet.WithMulticore(true))
	}
	if *tcpNoDelay {
		// Use TCPNoDelay (iota value 0) as defined in gnet package
		opts = append(opts, gnet.WithTCPNoDelay(gnet.TCPNoDelay))
	}

	opts = append(opts, gnet.WithSocketRecvBuffer(*bufferSize*1024))
	opts = append(opts, gnet.WithSocketSendBuffer(*bufferSize*1024))

	tcpTransport := client.NewTCPTransport(targetAddr, log, opts...)

	// Register the transport with the client
	if err := fdbClient.RegisterTransport("tcp", tcpTransport); err != nil {
		log.Fatal("Failed to register transport", "error", err)
	}

	// Register a success response handler for message type 1
	tcpTransport.RegisterHandler(client.MessageType(types.HandlerStatusSuccess.Byte()),
		func(c gnet.Conn, data []byte) error {
			// This handler is just used to avoid warnings - actual response processing
			// is done through the response channels
			log.Debug("Received success response", "size", len(data))
			return nil
		})

	// Start the client
	if err := fdbClient.Start(ctx); err != nil {
		log.Fatal("Failed to start client", "error", err)
	}
	log.Info("Client started successfully")

	// Run benchmark if requested
	if *benchmark {
		go func() {
			// Wait a bit for the connection to establish
			time.Sleep(2 * time.Second)

			// Configure benchmark
			benchCfg := BenchmarkConfig{
				Connections:    *connections,
				MessageSize:    *msgSize,
				MessageCount:   *msgCount,
				BatchSize:      *batchSize,
				Interval:       *msgInterval,
				WaitResponse:   *waitResponse,
				Timeout:        *timeout,
				HandlerType:    getHandlerType(),
				MaxConcurrency: *maxConcurrent,
			}

			// Run benchmark
			_, err := RunBenchmark(ctx, fdbClient, benchCfg, log)
			if err != nil {
				log.Error("Benchmark failed", "error", err)
			}
		}()
	} else if *sendMsg {
		// Send test messages if requested
		go func() {
			// Wait a bit for the connection to establish
			time.Sleep(2 * time.Second)

			for i := 0; i < *msgCount; i++ {
				if i > 0 {
					time.Sleep(*msgInterval)
				}

				if *waitResponse {
					sendWithResponse(ctx, fdbClient, *msgSize, i+1, log)
				} else {
					sendOneWay(ctx, fdbClient, *msgSize, i+1, log)
				}
			}
		}()
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down...")
	fdbClient.Close()
	log.Info("Client stopped")
}

// sendOneWay sends a message without waiting for a response
func sendOneWay(ctx context.Context, c *client.Client, size, msgNum int, log logger.Logger) {
	log.Info("Sending one-way message", "size", size, "message_number", msgNum)

	// Create test data
	data := generateTestData(size, msgNum)

	// Determine handler type based on msgType flag
	handlerType := getHandlerType()

	// Generate a message with the test data
	msg, err := messages.GenerateRandomMessageWithData(handlerType, data)
	if err != nil {
		log.Error("Failed to generate message", "error", err)
		return
	}

	encodedMsg, err := msg.Encode()
	if err != nil {
		log.Error("Failed to encode message", "error", err)
		return
	}

	// Send the message
	startTime := time.Now()
	if err := c.SendMessage("tcp", encodedMsg); err != nil {
		log.Error("Failed to send message", "error", err)
		return
	}

	// Log success
	elapsed := time.Since(startTime)
	log.Info("Message sent successfully",
		"size", size,
		"message_number", msgNum,
		"elapsed", elapsed,
		"throughput_MB/s", float64(size)/(1024*1024)/elapsed.Seconds())
}

// sendWithResponse sends a message and waits for a response
func sendWithResponse(ctx context.Context, c *client.Client, size, msgNum int, log logger.Logger) {
	log.Info("Sending message and waiting for response", "size", size, "message_number", msgNum)

	// Create test data
	data := generateTestData(size, msgNum)

	// Determine handler type based on msgType flag
	handlerType := getHandlerType()

	// Generate a message with the test data
	msg, err := messages.GenerateRandomMessageWithData(handlerType, data)
	if err != nil {
		log.Error("Failed to generate message", "error", err)
		return
	}

	encodedMsg, err := msg.Encode()
	if err != nil {
		log.Error("Failed to encode message", "error", err)
		return
	}

	// Get the TCP transport directly for manual handling
	transport, err := c.GetTransport("tcp")
	if err != nil {
		log.Error("Failed to get transport", "error", err)
		return
	}

	tcpTransport, ok := transport.(*client.TCPTransport)
	if !ok {
		log.Error("Transport is not a TCPTransport", "type", fmt.Sprintf("%T", transport))
		return
	}

	// Use the proper response type from types package
	responseType := client.MessageType(types.HandlerStatusSuccess.Byte())

	// Register a response channel before sending
	responseCh := tcpTransport.RegisterResponseChannel(responseType)

	// Send the message
	startTime := time.Now()
	if err := tcpTransport.Send(encodedMsg); err != nil {
		// Make sure to unregister the response channel on error
		tcpTransport.UnregisterResponseChannel(responseType)
		log.Error("Failed to send message", "error", err)
		return
	}

	// Wait for the response with the provided timeout
	response, err := tcpTransport.WaitForResponseWithTimeout(responseCh, *timeout)
	if err != nil {
		log.Error("Failed to receive response", "error", err)
		return
	}

	// Log success
	elapsed := time.Since(startTime)
	log.Info("Message round-trip completed",
		"size", size,
		"message_number", msgNum,
		"response_size", len(response),
		"elapsed", elapsed,
		"throughput_MB/s", float64(size)/(1024*1024)/elapsed.Seconds())
}

// generateTestData creates a byte slice filled with test data patterns
func generateTestData(size, seed int) []byte {
	data := make([]byte, size)

	// Add a timestamp and message number at the start of the data for tracing
	timestamp := time.Now().UnixNano()
	if size >= 12 {
		// Add timestamp (8 bytes) and message number (4 bytes) at the start
		binary.LittleEndian.PutUint64(data[0:8], uint64(timestamp))
		binary.LittleEndian.PutUint32(data[8:12], uint32(seed))

		// Fill the rest with a pattern
		for i := 12; i < size; i++ {
			data[i] = byte((i + seed) % 256)
		}
	} else {
		// If the buffer is too small, just fill it with a pattern
		for i := range data {
			data[i] = byte((i + seed) % 256)
		}
	}

	return data
}

// getHandlerType returns the appropriate handler type based on the msgType flag
func getHandlerType() types.HandlerType {
	switch *msgType {
	case "write":
		return types.WriteHandlerType // 'W' for WRITE
	case "read":
		return types.ReadHandlerType // 'R' for READ
	default:
		// Default to write handler if no valid type is specified
		return types.WriteHandlerType
	}
}
