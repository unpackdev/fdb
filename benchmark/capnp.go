package benchmark

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/unpackdev/fdb/pkg/logger"

	"capnproto.org/go/capnp/v3"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/pkg/db"
	"github.com/unpackdev/fdb/pkg/protocols/capn"
	"github.com/unpackdev/fdb/pkg/protocols/capn/schema"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// CapnpSuite implements the benchmark suite for Cap'n Proto protocol
type CapnpSuite struct {
	fdb             *fdb.FDB
	listener        net.Listener
	port            int
	server          *capn.Server
	db              db.Provider
	batchWriter     *db.BatchWriter
	pool            *sync.Pool // Buffer pool for reuse
	latencySampling int        // How often to sample latencies
	running         bool
}

// NewCapnpSuite initializes the CapnpSuite with buffer reuse and latency sampling settings.
func NewCapnpSuite(fdb *fdb.FDB, port int) *CapnpSuite {
	return &CapnpSuite{
		fdb:  fdb,
		port: port,
		pool: &sync.Pool{
			New: func() interface{} {
				// Dynamic buffer sizing - start with a small buffer
				return make([]byte, 4096)
			},
		},
		latencySampling: 10,
	}
}

// Start implements the Suite interface for the Cap'n Proto benchmark
func (cs *CapnpSuite) Start(ctx context.Context) error {
	logger.G().Info("Starting Cap'n Proto benchmark suite")

	var err error

	// Create the database provider
	cs.db, err = cs.createDbProvider(ctx)
	if err != nil {
		return err
	}
	zap.L().Info("Acquired database for Cap'n Proto benchmark")

	// Create batch writer
	batchSize := 1000
	flushInterval := 500 * time.Millisecond
	maxBatchSize := 10000
	cs.batchWriter = db.NewBatchWriter(cs.db.(*db.Db), batchSize, flushInterval, maxBatchSize)
	logger.G().Info("Created batch writer for Cap'n Proto benchmark",
		zap.Int("batchSize", batchSize),
		zap.Duration("flushInterval", flushInterval),
		zap.Int("maxBatchSize", maxBatchSize))

	// Create a TCP listener
	listenAddr := fmt.Sprintf(":%d", cs.port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	cs.listener = listener
	localAddr := listener.Addr().String()
	logger.G().Info("Cap'n Proto server listening for connections", zap.String("address", localAddr))

	// Create the Cap'n Proto server
	cs.server = capn.NewServer(cs.db, cs.batchWriter, logger.G())
	logger.G().Info("Cap'n Proto server initialized and ready")

	// Set the running flag
	cs.running = true

	// Start accepting connections
	go func() {
		logger.G().Info("Cap'n Proto server now accepting client connections")
		for cs.running {
			conn, err := listener.Accept()
			if err != nil {
				if !cs.running {
					return
				}
				logger.G().Error("Failed to accept connection", zap.Error(err))
				continue
			}

			// Set TCP options for better performance if possible
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				_ = tcpConn.SetNoDelay(true)
				_ = tcpConn.SetKeepAlive(true)
			}

			logger.G().Debug("New client connection accepted",
				zap.String("remoteAddr", conn.RemoteAddr().String()),
				zap.String("localAddr", conn.LocalAddr().String()))

			// Handle each connection in a separate goroutine
			go cs.handleConnection(ctx, conn)
		}
	}()

	// Sleep briefly to allow the server to fully start
	time.Sleep(500 * time.Millisecond)
	logger.G().Info("Cap'n Proto server is fully operational")

	return nil
}

// benchmarkConnAdapter adapts net.Conn to capn.Connection
type benchmarkConnAdapter struct {
	conn net.Conn
}

// Send implements the capn.Connection interface
func (c *benchmarkConnAdapter) Send(data []byte) error {
	_, err := c.conn.Write(data)
	return err
}

// handleConnection processes messages from a client connection
func (cs *CapnpSuite) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	// Create a connection adapter for the server
	connAdapter := &benchmarkConnAdapter{conn: conn}

	buffer := make([]byte, 4096) // Initial buffer size
	for {
		if !cs.running {
			return
		}

		// Read message
		n, err := conn.Read(buffer)
		if err != nil {
			if !cs.running {
				return
			}
			logger.G().Error("Failed to read from connection", zap.Error(err))
			return
		}

		if n == 0 {
			continue
		}

		// Convert TCP benchmark format to Cap'n Proto if needed
		var msgData []byte

		if len(buffer[:n]) >= 1 {
			actionByte := buffer[0]

			// If it's a write request in TCP format
			if actionByte == 'W' && n >= 34 {
				// Extract key and value from TCP format
				key := buffer[1:33]
				value := buffer[33:n]

				// Convert to Cap'n Proto request
				capnpMsg, err := capn.CreateSetRequest(key, value)
				if err != nil {
					logger.G().Error("Failed to create Cap'n Proto set request", zap.Error(err))
					conn.Write([]byte{0x01}) // Error code
					continue
				}

				msgData = capnpMsg
			} else if actionByte == 'R' && n >= 33 {
				// Extract key from TCP format
				key := buffer[1:33]

				// Convert to Cap'n Proto request
				capnpMsg, err := capn.CreateGetRequest(key)
				if err != nil {
					logger.G().Error("Failed to create Cap'n Proto get request", zap.Error(err))
					conn.Write([]byte("Error creating request"))
					continue
				}

				msgData = capnpMsg
			} else {
				// Use the raw data as it may already be a Cap'n Proto message
				msgData = buffer[:n]
			}
		} else {
			msgData = buffer[:n]
		}

		// Process message with the Cap'n Proto server
		cs.server.Handle(connAdapter, msgData)
	}
}

// createDbProvider creates a database provider for the benchmark
func (cs *CapnpSuite) createDbProvider(ctx context.Context) (db.Provider, error) {
	// Get benchmark database
	bDb, err := cs.fdb.GetDbManager().GetDb("benchmark")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve benchmark database: %w", err)
	}

	return bDb, nil
}

// Stop stops the Cap'n Proto server.
func (cs *CapnpSuite) Stop(ctx context.Context) error {
	logger.G().Info("Stopping Cap'n Proto benchmark server")
	cs.running = false

	if cs.listener != nil {
		if err := cs.listener.Close(); err != nil {
			logger.G().Error("Failed to close listener", zap.Error(err))
		}
	}

	logger.G().Info("Cap'n Proto benchmark server stopped")
	return nil
}

// AcquireClient creates and returns a new TCP client for Cap'n Proto communication.
func (cs *CapnpSuite) AcquireClient() (net.Conn, error) {
	listenAddr := fmt.Sprintf("127.0.0.1:%d", cs.port)
	serverAddr, err := net.ResolveTCPAddr("tcp", listenAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve server address")
	}

	// Connect to the server
	client, err := net.DialTCP("tcp", nil, serverAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Cap'n Proto server")
	}

	// Performance optimization: enable TCP_NODELAY to reduce latency
	if err := client.SetNoDelay(true); err != nil {
		logger.G().Warn("Failed to set TCP_NODELAY", zap.Error(err))
	}
	// Enable keep-alive to detect dead connections
	if err := client.SetKeepAlive(true); err != nil {
		logger.G().Warn("Failed to set TCP keep-alive", zap.Error(err))
	}

	return client, nil
}

// RunWriteBenchmark benchmarks writing messages through the Cap'n Proto server.
func (cs *CapnpSuite) RunWriteBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error {
	logger.G().Info("Running Cap'n Proto WRITE benchmark",
		zap.Int("clients", numClients),
		zap.Int("messagesPerClient", numMessagesPerClient))
	return cs.runBenchmark(ctx, numClients, numMessagesPerClient, report, true)
}

// RunReadBenchmark benchmarks reading messages from the Cap'n Proto server.
func (cs *CapnpSuite) RunReadBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error {
	logger.G().Info("Running Cap'n Proto READ benchmark",
		zap.Int("clients", numClients),
		zap.Int("messagesPerClient", numMessagesPerClient))
	return cs.runBenchmark(ctx, numClients, numMessagesPerClient, report, false)
}

// runBenchmark sends messages (writes or reads) and gathers benchmark results using goroutines.
func (cs *CapnpSuite) runBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report, isWrite bool) error {
	startTime := time.Now()
	var totalLatency atomic.Int64
	var successMessages atomic.Int64
	var failedMessages atomic.Int64

	// Track memory usage
	startMemory := getMemoryUsage()

	// Set the number of clients and messages per client in the report
	report.TotalClients = numClients
	report.MessagesPerClient = numMessagesPerClient

	// Initialize latency histogram storage
	latencyHistogram := make([]time.Duration, 0, numClients*numMessagesPerClient/cs.latencySampling)

	g, ctx := errgroup.WithContext(ctx)

	// For read-only benchmarking, ensure a key exists
	if !isWrite {
		logger.G().Info("Setting up test key for read benchmark")
		tmpClient, err := cs.AcquireClient()
		if err != nil {
			return err
		}
		defer tmpClient.Close()

		// Create a test key before running read benchmarks
		key := []byte("benchmark-key")
		value := []byte("benchmark-value")
		setRequest, err := capn.CreateSetRequest(key, value)
		if err != nil {
			return fmt.Errorf("failed to create set request: %w", err)
		}

		logger.G().Debug("Writing test key to server")
		// Write to the server
		_, err = tmpClient.Write(setRequest)
		if err != nil {
			return fmt.Errorf("failed to write set request: %w", err)
		}

		// Read response
		responseBuf := make([]byte, 4096)
		logger.G().Debug("Reading response for test key")
		n, err := tmpClient.Read(responseBuf)
		if err != nil {
			return fmt.Errorf("failed to read set response: %w", err)
		}

		// Validate the response
		err = cs.validateResponse(responseBuf[:n])
		if err != nil {
			return fmt.Errorf("invalid response: %w", err)
		}
		logger.G().Info("Test key setup complete for read benchmark")
	}

	// Create a mutex to protect appending to the latency histogram
	var latencyMutex sync.Mutex

	logger.G().Info("Starting benchmark clients",
		zap.Int("numClients", numClients),
		zap.Int("messagesPerClient", numMessagesPerClient))

	for i := 0; i < numClients; i++ {
		clientID := i
		g.Go(func() error {
			client, err := cs.AcquireClient()
			if err != nil {
				logger.G().Error("Failed to acquire client", zap.Error(err), zap.Int("clientID", clientID))
				return err
			}
			defer client.Close()

			// Get a buffer from the pool for this client
			reqBuffer := cs.pool.Get().([]byte)
			defer cs.pool.Put(reqBuffer)

			// Allocate response buffer once per client
			respBuffer := make([]byte, 4096)

			for j := 0; j < numMessagesPerClient; j++ {
				select {
				case <-ctx.Done():
					logger.G().Info("Context canceled, stopping benchmark execution")
					return ctx.Err()
				default:
					var err error
					messageStart := time.Now()

					var requestData []byte
					if isWrite {
						// Create a set request
						key := []byte(fmt.Sprintf("benchmark-key-%d-%d", clientID, j))
						value := []byte(fmt.Sprintf("benchmark-value-%d-%d", clientID, j))
						requestData, err = capn.CreateSetRequest(key, value)
					} else {
						// Create a get request
						key := []byte("benchmark-key")
						requestData, err = capn.CreateGetRequest(key)
					}

					if err != nil {
						failedMessages.Add(1)
						return fmt.Errorf("failed to create request: %w", err)
					}

					// Write the request to the server
					_, err = client.Write(requestData)
					if err != nil {
						failedMessages.Add(1)
						return fmt.Errorf("failed to write request: %w", err)
					}

					// Read the response from the server
					n, err := client.Read(respBuffer)
					if err != nil {
						failedMessages.Add(1)
						return fmt.Errorf("failed to read response: %w", err)
					}

					// Validate the response
					err = cs.validateResponse(respBuffer[:n])
					if err != nil {
						failedMessages.Add(1)
						return fmt.Errorf("invalid response: %w", err)
					}

					latency := time.Since(messageStart)
					successMessages.Add(1)
					totalLatency.Add(latency.Nanoseconds())

					// Sample latencies based on sampling rate
					if j%cs.latencySampling == 0 {
						latencyMutex.Lock()
						latencyHistogram = append(latencyHistogram, latency)
						latencyMutex.Unlock()
					}
				}
			}
			return nil
		})
	}

	logger.G().Info("Waiting for all benchmark clients to complete")
	// Wait for all clients to finish
	if err := g.Wait(); err != nil {
		logger.G().Error("Error during benchmark execution", zap.Error(err))
		return err
	}

	logger.G().Info("Benchmark completed, preparing report")
	// Update the report after all clients have finished
	report.SuccessMessages = int(successMessages.Load())
	report.FailedMessages = int(failedMessages.Load())
	report.TotalMessages = int(successMessages.Load()) + int(failedMessages.Load())
	report.TotalDuration = time.Since(startTime)
	report.Throughput = float64(report.SuccessMessages) / report.TotalDuration.Seconds()

	// Calculate average latency
	if successMessages.Load() > 0 {
		report.AvgLatency = time.Duration(totalLatency.Load() / successMessages.Load())
	}

	// Calculate jitter
	report.Jitter = calculateStdDev(latencyHistogram)
	report.LatencyHistogram = latencyHistogram

	// Get memory usage
	endMemory := getMemoryUsage()
	report.MemoryUsed = endMemory - startMemory

	// Finalize the report to calculate percentile latencies and other metrics
	report.Finalize()

	logger.G().Info("Benchmark report generated",
		zap.Int("successMessages", report.SuccessMessages),
		zap.Int("failedMessages", report.FailedMessages),
		zap.Duration("avgLatency", report.AvgLatency),
		zap.Float64("throughput", report.Throughput))

	return nil
}

// validateResponse validates a Cap'n Proto response
func (cs *CapnpSuite) validateResponse(responseData []byte) error {
	// If the response is a simple success code (similar to TCP handler responses)
	if len(responseData) == 1 && responseData[0] == 0x00 {
		return nil // Success code from raw TCP handler
	}

	// Try to parse as Cap'n Proto message
	// Unmarshal the message
	msg, err := capnp.Unmarshal(responseData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Read the root response object
	resp, err := schema.ReadRootResponse(msg)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error responses
	if resp.Which() == schema.Response_Which_error {
		errMsg := resp.Error()
		return fmt.Errorf("server returned error: %s", errMsg)
	}

	return nil
}
