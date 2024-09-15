package cmd

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/types"
	"github.com/urfave/cli/v2"
	"io"
	"runtime"
	"time"

	"github.com/quic-go/quic-go"
)

// BenchmarkCommand returns a cli.Command that benchmarks the real client
func BenchmarkCommand() *cli.Command {
	return &cli.Command{
		Name:  "benchmark",
		Usage: "Benchmark the real client",
		Action: func(c *cli.Context) error {
			fmt.Println("Running client benchmark...")

			// Configure QUIC transport
			cnf := config.Config{
				MdbxNodes: []config.MdbxNode{
					{
						Name: "benchmark",
						Path: "/tmp/",
					},
				},
				Transports: []config.Transport{
					{
						Type:    types.QUICTransportType,
						Enabled: true,
						Config: config.QuicTransport{
							Enabled: true,
							IPv4:    "127.0.0.1",
							Port:    4433,
							TLS: config.TLS{
								Key:    "./data/certs/key.pem",
								Cert:   "./data/certs/cert.pem",
								RootCA: "",
							},
						},
					},
				},
			}

			// Initialize FDB
			fdbc, err := fdb.New(c.Context, cnf)
			if err != nil {
				return err
			}

			// Get QUIC transport
			quicTransport, err := fdbc.GetTransportByType(types.QUICTransportType)
			if err != nil {
				return errors.Wrap(err, "failed to get retrieve quic transport")
			}

			// Start the QUIC server
			quicServer, ok := quicTransport.(*fdb.QuicServer)
			if !ok {
				return errors.New("failed to cast transport to QuicServer")
			}

			// Gets benchmarks temporary database...
			db, err := fdbc.GetDbManager().GetDb("benchmark")
			if err != nil {
				return err
			}

			wHandler := fdb.NewQuicWriteHandler(db)
			quicServer.RegisterHandler(fdb.WriteHandlerType, wHandler.HandleMessage)

			rHandler := fdb.NewQuicReadHandler(db)
			quicServer.RegisterHandler(fdb.ReadHandlerType, rHandler.HandleMessage)

			if err := quicServer.Start(); err != nil {
				return fmt.Errorf("failed to start QUIC server: %w", err)
			}

			fmt.Println("QUIC server started successfully")

			// Benchmark client
			err = benchmarkQuicClient(quicServer.Addr())
			if err != nil {
				return fmt.Errorf("client benchmark failed: %w", err)
			}

			return nil
		},
	}
}

// benchmarkQuicClient benchmarks the QUIC client by simulating write and read operations
func benchmarkQuicClient(serverAddr string) error {
	// Setup TLS configuration for QUIC client
	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-example"},
	}

	// Start benchmarking
	start := time.Now()
	var memStart runtime.MemStats
	runtime.ReadMemStats(&memStart)

	// Connect to the QUIC server
	client, err := quic.DialAddr(context.Background(), serverAddr, clientTLSConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to dial QUIC server: %w", err)
	}
	defer client.CloseWithError(0, "closing connection")

	// Open stream
	stream, err := client.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Simulate write operation
	message := createWriteMessage()
	encodedMessage, err := message.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	_, err = stream.Write(encodedMessage)
	if err != nil {
		return fmt.Errorf("failed to write message to server: %w", err)
	}

	// Read server response
	buffer := make([]byte, 1024)
	_, err = stream.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	fmt.Printf("Response from server: %s\n", string(buffer))

	// Simulate read operation
	readMessage := createReadMessage(message.Key)
	encodedReadMessage, err := readMessage.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode read message: %w", err)
	}

	_, err = stream.Write(encodedReadMessage)
	if err != nil {
		return fmt.Errorf("failed to write read message to server: %w", err)
	}

	// Read back the data length
	_, err = io.ReadFull(stream, buffer[:4])
	if err != nil {
		return fmt.Errorf("failed to read data length: %w", err)
	}
	valueLength := binary.BigEndian.Uint32(buffer[:4])

	// Read the actual value
	readBuffer := make([]byte, valueLength)
	_, err = io.ReadFull(stream, readBuffer)
	if err != nil {
		return fmt.Errorf("failed to read value: %w", err)
	}
	fmt.Printf("Data read from server: %s\n", string(readBuffer))

	// Capture memory usage after the operation
	var memEnd runtime.MemStats
	runtime.ReadMemStats(&memEnd)

	// Calculate elapsed time
	elapsed := time.Since(start)
	fmt.Printf("Benchmark completed in %s\n", elapsed)
	fmt.Printf("Memory used: %d bytes\n", memEnd.Alloc-memStart.Alloc)

	return nil
}

// createWriteMessage generates a random write message
func createWriteMessage() fdb.Message {
	var key [32]byte
	rand.Read(key[:])
	return fdb.Message{
		Handler: fdb.WriteHandlerType,
		Key:     key,
		Data:    []byte("benchmark test data"),
	}
}

// createReadMessage generates a read message for a given key
func createReadMessage(key [32]byte) fdb.Message {
	return fdb.Message{
		Handler: fdb.ReadHandlerType,
		Key:     key,
	}
}
