package benchmark

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/db"
	transport_quic "github.com/unpackdev/fdb/transports/quic"
	"github.com/unpackdev/fdb/types"
	"golang.org/x/sync/errgroup"
	"sync"
	"sync/atomic"
	"time"
)

// QuicSuite represents the QUIC-specific benchmark suite.
type QuicSuite struct {
	fdb             *fdb.FDB
	quicServer      *transport_quic.Server
	client          quic.Connection
	stream          quic.Stream
	pool            *sync.Pool // Buffer pool for reuse
	latencySampling int        // How often to sample latencies (e.g., every 1000th message)
}

// NewQuicSuite initializes the QuicSuite with buffer reuse and latency sampling settings.
func NewQuicSuite(fdb *fdb.FDB, latencySampling int) *QuicSuite {
	return &QuicSuite{
		fdb: fdb,
		pool: &sync.Pool{
			New: func() interface{} {
				// Dynamic buffer sizing - start with a small buffer
				return make([]byte, 64) // Default buffer size
			},
		},
		latencySampling: latencySampling,
	}
}

// Start starts the QUIC server for benchmarking.
func (qs *QuicSuite) Start(ctx context.Context) error {
	quicTransport, err := qs.fdb.GetTransportByType(types.QUICTransportType)
	if err != nil {
		return fmt.Errorf("failed to retrieve QUIC transport: %w", err)
	}

	quicServer, ok := quicTransport.(*transport_quic.Server)
	if !ok {
		return fmt.Errorf("failed to cast transport to QuicServer")
	}

	bDb, err := qs.fdb.GetDbManager().GetDb("benchmark")
	if err != nil {
		return fmt.Errorf("failed to retrieve benchmark database: %w", err)
	}

	// Create a new BatchWriter with a batch size of 512 and flush interval of 1 second
	batchWriter := db.NewBatchWriter(bDb.(*db.Db), 512, 500*time.Millisecond, 15)

	wHandler := transport_quic.NewQuicWriteHandler(bDb, batchWriter)
	quicServer.RegisterHandler(types.WriteHandlerType, wHandler.HandleMessage)

	rHandler := transport_quic.NewQuicReadHandler(bDb)
	quicServer.RegisterHandler(types.ReadHandlerType, rHandler.HandleMessage)

	if err := quicServer.Start(); err != nil {
		return fmt.Errorf("failed to start QUIC server: %w", err)
	}

	qs.quicServer = quicServer
	fmt.Println("QUIC server started successfully")
	return nil
}

// Stop stops the QUIC server and closes the client connection and stream.
func (qs *QuicSuite) Stop(ctx context.Context) error {
	if qs.quicServer != nil {
		if err := qs.quicServer.Stop(); err != nil {
			return err
		}
	}

	fmt.Println("QUIC server stopped successfully")
	return nil
}

// AcquireClient sets up a QUIC client and stream.
func (qs *QuicSuite) AcquireClient(ctx context.Context) (quic.Connection, quic.Stream, error) {
	serverAddr := qs.quicServer.Addr()

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	client, err := quic.DialAddr(ctx, serverAddr, clientTLSConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial QUIC server: %w", err)
	}

	stream, err := client.OpenStreamSync(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open stream: %w", err)
	}

	return client, stream, nil
}

// RunWriteBenchmark benchmarks writing messages through the QUIC server.
func (qs *QuicSuite) RunWriteBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error {
	return qs.runBenchmark(ctx, numClients, numMessagesPerClient, report, true)
}

// RunReadBenchmark benchmarks reading messages from the QUIC server.
func (qs *QuicSuite) RunReadBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report) error {
	return qs.runBenchmark(ctx, numClients, numMessagesPerClient, report, false)
}

// runBenchmark sends messages (writes or reads) and gathers benchmark results using goroutines.
func (qs *QuicSuite) runBenchmark(ctx context.Context, numClients int, numMessagesPerClient int, report *Report, isWrite bool) error {
	startTime := time.Now()
	var totalLatency time.Duration
	var successMessages int64
	var failedMessages int64

	report.TotalClients = numClients
	report.MessagesPerClient = numMessagesPerClient

	g, ctx := errgroup.WithContext(ctx)

	for i := 1; i <= numClients; i++ {
		g.Go(func() error {
			client, stream, err := qs.AcquireClient(ctx)
			if err != nil {
				return err
			}
			defer client.CloseWithError(0, "closing connection")
			defer stream.Close()

			for j := 0; j < numMessagesPerClient; j++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// Retrieve a buffer from the pool
					buf := qs.pool.Get().([]byte)

					messageStart := time.Now()
					var err error
					if isWrite {
						// Create and encode the write message (reusing the buffer)
						message := createWriteMessage()
						encodedMessage, err := message.EncodeWithBuffer(buf)
						if err != nil {
							// Return the buffer to the pool on error
							qs.pool.Put(buf)
							return fmt.Errorf("failed to encode message: %w", err)
						}

						// Write the message to the QUIC server
						_, err = stream.Write(encodedMessage)
						if err != nil {
							atomic.AddInt64(&failedMessages, 1)
							qs.pool.Put(buf)
							return fmt.Errorf("failed to write QUIC message: %w", err)
						}

						// Prepare a buffer for reading the response
						responseBuf := make([]byte, len(encodedMessage)) // Adjust size as per response
						_, err = stream.Read(responseBuf)
						if err != nil {
							atomic.AddInt64(&failedMessages, 1)
							qs.pool.Put(buf)
							return fmt.Errorf("failed to read QUIC response: %w", err)
						}
					}

					latency := time.Since(messageStart)
					if err != nil {
						atomic.AddInt64(&failedMessages, 1)
					} else {
						atomic.AddInt64(&successMessages, 1)
						totalLatency += latency
						if j%qs.latencySampling == 0 {
							report.LatencyHistogram = append(report.LatencyHistogram, latency)
						}
					}

					qs.pool.Put(buf) // Return the buffer to the pool
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Update the report after all clients have finished
	report.Jitter = calculateStdDev(report.LatencyHistogram)
	report.SuccessMessages = int(successMessages)
	report.FailedMessages = int(failedMessages)
	report.TotalMessages = int(successMessages) + int(failedMessages)
	report.TotalDuration = time.Since(startTime)
	report.Throughput = float64(successMessages) / report.TotalDuration.Seconds()

	// Finalize the report for additional statistics
	report.Finalize()

	return nil
}
