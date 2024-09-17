package benchmark

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/db"
	transport_tcp "github.com/unpackdev/fdb/transports/tcp"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TcpSuite represents the benchmarking suite for TCP with buffer reuse and latency sampling.
type TcpSuite struct {
	fdb             *fdb.FDB
	server          *transport_tcp.Server
	pool            *sync.Pool // Buffer pool for reuse
	latencySampling int        // How often to sample latencies (e.g., every 1000th message)
}

// NewTcpSuite initializes the TcpSuite with buffer reuse and latency sampling settings.
func NewTcpSuite(fdb *fdb.FDB, latencySampling int) *TcpSuite {
	return &TcpSuite{
		fdb: fdb,
		pool: &sync.Pool{
			New: func() interface{} {
				// Dynamic buffer sizing - start with a small buffer
				return make([]byte, 64) // Default buffer size is 64 bytes, will grow as needed
			},
		},
		latencySampling: latencySampling,
	}
}

// Start starts the TCP server for benchmarking.
func (ts *TcpSuite) Start(ctx context.Context) error {
	tcpTransport, err := ts.fdb.GetTransportByType(types.TCPTransportType)
	if err != nil {
		return fmt.Errorf("failed to retrieve TCP transport: %w", err)
	}

	tcpServer, ok := tcpTransport.(*transport_tcp.Server)
	if !ok {
		return fmt.Errorf("failed to cast transport to TcpServer")
	}

	bDb, err := ts.fdb.GetDbManager().GetDb("benchmark")
	if err != nil {
		return fmt.Errorf("failed to retrieve benchmark database: %w", err)
	}

	// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
	batchWriter := db.NewBatchWriter(bDb.(*db.Db), 512, 500*time.Millisecond, 15)

	wHandler := transport_tcp.NewTCPWriteHandler(bDb, batchWriter)
	tcpServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

	rHandler := transport_tcp.NewTCPReadHandler(bDb)
	tcpServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

	if sErr := tcpServer.Start(ctx); sErr != nil {
		zap.L().Error("failed to start TCP transport", zap.Error(sErr))
	}

	ts.server = tcpServer
	zap.L().Info("TCP transport is ready to accept traffic", zap.String("addr", tcpServer.Addr()))
	return nil
}

// Stop stops the TCP server and closes the client connection.
func (ts *TcpSuite) Stop(ctx context.Context) error {
	if ts.server != nil {
		if err := ts.server.Stop(); err != nil {
			return err
		}
	}
	zap.L().Info("TCP transport stopped successfully")
	return nil
}

// AcquireClient creates and returns a new TCP client.
func (ts *TcpSuite) AcquireClient() (*net.TCPConn, error) {
	// Resolve the server address
	serverAddr, err := net.ResolveTCPAddr("tcp", ts.server.Addr())
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve server address")
	}

	// Create the TCP client
	client, err := net.DialTCP("tcp", nil, serverAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to server")
	}

	return client, nil
}

// RunWriteBenchmark benchmarks writing messages through the TCP server.
func (ts *TcpSuite) RunWriteBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error {
	return ts.runBenchmark(ctx, numClients, numMessagesPerClient, report, true)
}

// RunReadBenchmark benchmarks reading messages from the TCP server.
func (ts *TcpSuite) RunReadBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error {
	return ts.runBenchmark(ctx, numClients, numMessagesPerClient, report, false)
}

// runBenchmark sends messages (writes or reads) and gathers benchmark results using goroutines.
func (ts *TcpSuite) runBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report, isWrite bool) error {
	startTime := time.Now()
	var totalLatency time.Duration
	var successMessages int64
	var failedMessages int64

	// Set the number of clients and messages per client in the report
	report.TotalClients = numClients
	report.MessagesPerClient = numMessagesPerClient

	g, ctx := errgroup.WithContext(ctx)

	for i := 1; i <= numClients; i++ {
		g.Go(func() error {
			client, err := ts.AcquireClient()
			if err != nil {
				return err
			}
			defer client.Close()

			for j := 0; j < numMessagesPerClient; j++ {
				select {
				case <-ctx.Done():
					zap.L().Info("Context canceled, stopping benchmark execution")
					return ctx.Err()
				default:
					// Retrieve a buffer from the pool
					buf := ts.pool.Get().([]byte)

					var err error
					var latency time.Duration
					messageStart := time.Now()

					if isWrite {
						// Create and encode the write message (reusing the buffer)
						message := createWriteMessage()
						encodedMessage, err := message.EncodeWithBuffer(buf)
						if err != nil {
							// Return the buffer to the pool on error
							ts.pool.Put(buf)
							return fmt.Errorf("failed to encode message: %w", err)
						}

						// Write the message to the server
						_, err = client.Write(encodedMessage)
						if err != nil {
							atomic.AddInt64(&failedMessages, 1)
							ts.pool.Put(buf)
							return errors.Wrap(err, "failed to write TCP message")
						}

						// Read the response from the server
						responseBuf := make([]byte, len(encodedMessage)) // Adjust size as per response
						_, err = client.Read(responseBuf)
						if err != nil {
							atomic.AddInt64(&failedMessages, 1)
							ts.pool.Put(buf)
							return errors.Wrap(err, "failed to read TCP response")
						}

					}

					if !isWrite { // For read-only benchmarking
						// Read the response from the server
						responseBuf := make([]byte, 1024) // Adjust size as per response
						_, err = client.Read(responseBuf)
						if err != nil {
							atomic.AddInt64(&failedMessages, 1)
							ts.pool.Put(buf)
							return errors.Wrap(err, "failed to read TCP response")
						}
					}

					latency = time.Since(messageStart)

					if err != nil {
						atomic.AddInt64(&failedMessages, 1)
						ts.pool.Put(buf)
						return errors.Wrap(err, "failed to write/read TCP message")
					} else {
						atomic.AddInt64(&successMessages, 1)
						totalLatency += latency

						// Sample latencies
						if j%ts.latencySampling == 0 {
							report.LatencyHistogram = append(report.LatencyHistogram, latency)
						}
					}

					// Return the buffer to the pool for reuse
					ts.pool.Put(buf)
				}
			}
			return nil
		})
	}

	// Wait for all clients to finish
	if err := g.Wait(); err != nil {
		return err
	}

	// Calculate jitter (standard deviation of latencies)
	report.Jitter = calculateStdDev(report.LatencyHistogram)

	// Update report after all clients have finished
	report.SuccessMessages = int(successMessages)
	report.FailedMessages = int(failedMessages)
	report.TotalMessages = int(successMessages) + int(failedMessages)
	report.TotalDuration = time.Since(startTime)
	report.Throughput = float64(successMessages) / report.TotalDuration.Seconds()

	// Finalize the report to calculate average latency and other metrics
	report.Finalize()

	return nil
}
