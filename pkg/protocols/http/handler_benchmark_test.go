// handler_benchmark_test.go
package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"
	"github.com/unpackdev/fdb/pkg/transports/tcp"
	"github.com/unpackdev/fdb/pkg/types"
	"go.uber.org/zap"

	"github.com/stretchr/testify/require"
)

// setupBenchmarkServer initializes the HTTP server for benchmarking.
func setupBenchmarkServer(t testing.TB, ctx context.Context, cfg config.TcpTransport) (*tcp.Server, logger.Logger, *observability.Observability, string, error) {
	// Get a free port
	port := tcp.GetFreePortForTest(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Setup server using existing test helper
	gLog, obs, server, err := setupBenchmarkServerInternal(t, ctx, cfg, addr)
	if err != nil {
		return nil, nil, nil, "", err
	}

	return server, gLog, obs, addr, nil
}

// setupBenchmarkServerInternal is an internal helper to setup the server with a specific address.
func setupBenchmarkServerInternal(t testing.TB, ctx context.Context, cfg config.TcpTransport, addr string) (logger.Logger, *observability.Observability, *tcp.Server, error) {
	// Parse the address to extract the port
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid address format: %v", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid port: %v", err)
	}

	// Create TCP transport configuration without TLS for testing
	tcpConfig := cfg
	tcpConfig.IPv4 = net.ParseIP(host).String()
	tcpConfig.Port = port

	gLog, obs, server := tcp.SetupServerTest(t, ctx, tcpConfig)

	// Initialize and Register HTTP custom OnTraffic handler
	httpTrafficHandler := NewHTTPTrafficHandler(gLog)
	server.SetOnTrafficHandler(httpTrafficHandler.Handle)

	// Start the server asynchronously
	go func() {
		if err := server.Start(ctx); err != nil {
			// Handle startup error (you might want to log this)
			gLog.Error("Failed to start TCP server", zap.Error(err))
		}
	}()

	// Wait until the server has started
	select {
	case <-server.WaitStarted():
		// Server has started
	case <-time.After(5 * time.Second):
		return nil, nil, nil, fmt.Errorf("TCP server did not start in time")
	}

	return gLog, obs, server, nil
}

// BenchmarkHTTPHandler_GET_Root benchmarks the GET / endpoint.
func BenchmarkHTTPHandler_GET_Root(b *testing.B) {
	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the HTTP server
	cfg := config.TcpTransport{
		Type:    types.TCPTransportType,
		Enabled: true,
		IPv4:    net.ParseIP("127.0.0.1").String(),
		Port:    0,   // Set to 0 to let the setupBenchmarkServer assign a free port
		TLS:     nil, // Disable TLS for simplicity
	}

	server, _, _, addr, err := setupBenchmarkServer(b, ctx, cfg)
	require.NoError(b, err, "Failed to setup benchmark server")
	defer func() {
		if err := server.Stop(); err != nil {
			b.Fatalf("Failed to stop server: %v", err)
		}
	}()

	// Define the GET / request
	request := "GET / HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Connection: close\r\n" +
		"\r\n"

	expectedResponse := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Length: 27\r\n" +
		"\r\n" +
		"Welcome to the HTTP Server!"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Establish a TCP connection to the server
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatalf("Failed to connect to server: %v", err)
		}

		// Send the GET request
		_, err = conn.Write([]byte(request))
		if err != nil {
			conn.Close()
			b.Fatalf("Failed to send request: %v", err)
		}

		// Read the response
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		responseBuf := new(bytes.Buffer)
		_, err = io.Copy(responseBuf, conn)
		if err != nil && err != io.EOF {
			conn.Close()
			b.Fatalf("Failed to read response: %v", err)
		}

		response := responseBuf.String()

		// Validate the response
		if response != expectedResponse {
			conn.Close()
			b.Fatalf("Unexpected response:\nExpected:\n%s\nGot:\n%s", expectedResponse, response)
		}

		conn.Close()
	}
}

// BenchmarkHTTPHandler_POST_Echo benchmarks the POST /echo endpoint.
func BenchmarkHTTPHandler_POST_Echo(b *testing.B) {
	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the HTTP server
	cfg := config.TcpTransport{
		Type:    types.TCPTransportType,
		Enabled: true,
		IPv4:    net.ParseIP("127.0.0.1").String(),
		Port:    tcp.GetFreePortForTest(b),
		TLS:     nil,
	}

	server, _, _, addr, err := setupBenchmarkServer(b, ctx, cfg)
	require.NoError(b, err, "Failed to setup benchmark server")
	defer func() {
		if err := server.Stop(); err != nil {
			b.Fatalf("Failed to stop server: %v", err)
		}
	}()

	// Define the POST /echo request
	body := `{"message":"Hello, Echo!"}`
	contentLength := len(body)
	request := fmt.Sprintf(
		"POST /echo HTTP/1.1\r\n"+
			"Host: localhost\r\n"+
			"Content-Type: application/json\r\n"+
			"Content-Length: %d\r\n"+
			"Connection: close\r\n"+
			"\r\n%s",
		contentLength,
		body,
	)

	expectedResponse := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Length: 12\r\n" +
		"\r\n" +
		"Hello, Echo!"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Establish a TCP connection to the server
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatalf("Failed to connect to server: %v", err)
		}

		// Send the POST request
		_, err = conn.Write([]byte(request))
		if err != nil {
			conn.Close()
			b.Fatalf("Failed to send request: %v", err)
		}

		// Read the response
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		responseBuf := new(bytes.Buffer)
		_, err = io.Copy(responseBuf, conn)
		if err != nil && err != io.EOF {
			conn.Close()
			b.Fatalf("Failed to read response: %v", err)
		}

		response := responseBuf.String()

		// Validate the response
		if response != expectedResponse {
			conn.Close()
			b.Fatalf("Unexpected response:\nExpected:\n%s\nGot:\n%s", expectedResponse, response)
		}

		conn.Close()
	}
}

// BenchmarkHTTPHandler_Concurrent benchmarks the handler with concurrent clients.
func BenchmarkHTTPHandler_Concurrent(b *testing.B) {
	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the HTTP server
	cfg := config.TcpTransport{
		Type:    types.TCPTransportType,
		Enabled: true,
		IPv4:    net.ParseIP("127.0.0.1").String(),
		Port:    tcp.GetFreePortForTest(b),
		TLS:     nil,
	}

	server, _, _, addr, err := setupBenchmarkServer(b, ctx, cfg)
	require.NoError(b, err, "Failed to setup benchmark server")
	defer func() {
		if err := server.Stop(); err != nil {
			b.Fatalf("Failed to stop server: %v", err)
		}
	}()

	// Define the GET / request
	request := "GET / HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Connection: close\r\n" +
		"\r\n"

	expectedResponse := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Length: 27\r\n" +
		"\r\n" +
		"Welcome to the HTTP Server!"

	// Number of concurrent clients
	concurrency := 100

	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Create a buffered channel to distribute work
	jobs := make(chan int, b.N)

	// Populate the jobs channel
	for i := 0; i < b.N; i++ {
		jobs <- i
	}
	close(jobs)

	// Start concurrent workers
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for range jobs {
				// Establish a TCP connection to the server
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					b.Fatalf("Failed to connect to server: %v", err)
				}

				// Send the GET request
				_, err = conn.Write([]byte(request))
				if err != nil {
					conn.Close()
					b.Fatalf("Failed to send request: %v", err)
				}

				// Read the response
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				responseBuf := new(bytes.Buffer)
				_, err = io.Copy(responseBuf, conn)
				if err != nil && err != io.EOF {
					conn.Close()
					b.Fatalf("Failed to read response: %v", err)
				}

				response := responseBuf.String()

				// Validate the response
				if response != expectedResponse {
					conn.Close()
					b.Fatalf("Unexpected response:\nExpected:\n%s\nGot:\n%s", expectedResponse, response)
				}

				conn.Close()
			}
		}()
	}

	wg.Wait()
}
