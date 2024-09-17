package benchmark

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	transport_dummy "github.com/unpackdev/fdb/transports/dummy"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// DummySuite represents the benchmarking suite with buffer reuse and lighter LatencyHistogram.
type DummySuite struct {
	fdb             *fdb.FDB
	server          *transport_dummy.Server
	pool            *sync.Pool // Buffer pool for reuse
	latencySampling int        // How often to sample latencies (e.g., every 1000th message)
}

// NewDummySuite initializes the DummySuite with buffer reuse and latency sampling settings.
func NewDummySuite(fdb *fdb.FDB, latencySampling int) *DummySuite {
	return &DummySuite{
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

// Start starts the Dummy server for benchmarking.
func (ds *DummySuite) Start(ctx context.Context) error {
	dummyTransport, err := ds.fdb.GetTransportByType(types.DummyTransportType)
	if err != nil {
		return fmt.Errorf("failed to retrieve dummy transport: %w", err)
	}

	dummyServer, ok := dummyTransport.(*transport_dummy.Server)
	if !ok {
		return fmt.Errorf("failed to cast transport to DummyServer")
	}

	db, err := ds.fdb.GetDbManager().GetDb("benchmark")
	if err != nil {
		return fmt.Errorf("failed to retrieve benchmark database: %w", err)
	}

	wHandler := transport_dummy.NewDummyWriteHandler(db)
	dummyServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

	rHandler := transport_dummy.NewDummyReadHandler(db)
	dummyServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

	if sErr := dummyServer.Start(ctx); sErr != nil {
		zap.L().Error(
			"failed to start dummy transport",
			zap.Error(sErr),
		)
	}

	ds.server = dummyServer
	zap.L().Info("Dummy transport is ready to accept the traffic", zap.String("addr", dummyServer.Addr()))
	return nil
}

// Stop stops the Dummy server and closes the client connection and stream.
func (ds *DummySuite) Stop(ctx context.Context) error {
	if ds.server != nil {
		if err := ds.server.Stop(); err != nil {
			return err
		}
	}
	zap.L().Info("Dummy transport stopped successfully")
	return nil
}

// AcquireClient creates and returns a new UDP client.
func (ds *DummySuite) AcquireClient() (*net.UDPConn, error) {
	// Resolve the server address
	serverAddr, err := net.ResolveUDPAddr("udp", ds.server.Addr())
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve server address")
	}

	// Create the UDP client
	client, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to server")
	}

	return client, nil
}

// RunWriteBenchmark benchmarks writing messages through the Dummy server.
func (ds *DummySuite) RunWriteBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error {
	return ds.runBenchmark(ctx, numClients, numMessagesPerClient, report, true)
}

// RunReadBenchmark benchmarks reading messages from the Dummy server.
func (ds *DummySuite) RunReadBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error {
	return ds.runBenchmark(ctx, numClients, numMessagesPerClient, report, false)
}

// runBenchmark sends messages (writes or reads) and gathers benchmark results using goroutines.
func (ds *DummySuite) runBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report, isWrite bool) error {
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
			client, err := ds.AcquireClient()
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
					buf := ds.pool.Get().([]byte)

					var err error
					var latency time.Duration
					messageStart := time.Now()

					if isWrite {
						// Create and encode the write message (reusing the buffer)
						message := createWriteMessage()
						encodedMessage, err := message.EncodeWithBuffer(buf)
						if err != nil {
							// Return the buffer to the pool on error
							ds.pool.Put(buf)
							return fmt.Errorf("failed to encode message: %w", err)
						}
						_, err = client.Write(encodedMessage)
					} else {
						// Simulate a read request (e.g., sending a read message)
						message := createReadMessage([32]byte{})
						encodedMessage, err := message.EncodeWithBuffer(buf)
						if err != nil {
							ds.pool.Put(buf)
							return fmt.Errorf("failed to encode message: %w", err)
						}
						_, err = client.Write(encodedMessage)
					}

					latency = time.Since(messageStart)

					if err != nil {
						atomic.AddInt64(&failedMessages, 1)
						ds.pool.Put(buf)
						return errors.Wrap(err, "failed to write/read dummy message")
					} else {
						atomic.AddInt64(&successMessages, 1)
						totalLatency += latency

						// Sample latencies
						if j%ds.latencySampling == 0 {
							report.LatencyHistogram = append(report.LatencyHistogram, latency)
						}
					}

					// Return the buffer to the pool for reuse
					ds.pool.Put(buf)
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
