// handler_test.go
package http

import (
	"bufio"
	"bytes"
	"context"
	"fmt"

	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"
	"github.com/unpackdev/fdb/pkg/transports/tcp"
	"github.com/unpackdev/fdb/pkg/types"

	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupHTTPServerTest(t testing.TB, ctx context.Context) (*tcp.Server, logger.Logger, *observability.Observability, string) {
	// Get a free port
	port := tcp.GetFreePortForTest(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Create TCP transport configuration without TLS for testing
	tcpConfig := config.TcpTransport{
		Type:    types.TCPTransportType,
		Enabled: true,
		IPv4:    net.ParseIP("127.0.0.1").String(),
		Port:    port, // Use the dynamically assigned port
		TLS:     nil,  // Disable TLS for simplicity
	}

	gLog, obs, server := tcp.SetupServerTest(t, ctx, tcpConfig)

	// Initialize and Register HTTP custom OnTraffic handler
	httpTrafficHandler := NewHTTPTrafficHandler(gLog)
	server.SetOnTrafficHandler(httpTrafficHandler.Handle)

	// Start the server
	go func() {
		if err := server.Start(ctx); err != nil {
			t.Fatalf("Failed to start TCP server: %v", err)
		}
	}()

	// Wait until the server has started
	select {
	case <-server.WaitStarted():
		// Server has started
	case <-time.After(5 * time.Second): // Increased timeout to 5 seconds for better reliability
		t.Fatalf("TCP server did not start in time")
	}

	return server, gLog, obs, addr
}

func TestHTTPHandler(t *testing.T) {
	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the HTTP server
	server, _, _, addr := setupHTTPServerTest(t, ctx)
	defer func() {
		if err := server.Stop(); err != nil {
			t.Fatalf("Failed to stop server: %v", err)
		}
	}()

	// Define test cases
	tests := []struct {
		name            string
		request         string
		expectedStatus  int
		expectedBody    string
		expectedHeaders http.Header
	}{
		{
			name:           "GET Root",
			request:        "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n",
			expectedStatus: http.StatusOK,
			expectedBody:   "Welcome to the HTTP Server!",
			expectedHeaders: http.Header{
				"Content-Type":   {"text/plain; charset=utf-8"},
				"Content-Length": {"27"},
			},
		},
		{
			name: "POST Echo",
			request: "POST /echo HTTP/1.1\r\n" +
				"Host: localhost\r\n" +
				"Content-Type: application/json\r\n" +
				"Content-Length: 26\r\n" +
				"Connection: close\r\n" +
				"\r\n" +
				`{"message":"Hello, Echo!"}`,
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, Echo!",
			expectedHeaders: http.Header{
				"Content-Type":   {"text/plain; charset=utf-8"},
				"Content-Length": {"12"},
			},
		},
		{
			name:           "Unknown Route",
			request:        "GET /unknown HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Not Found",
			expectedHeaders: http.Header{
				"Content-Type":   {"text/plain; charset=utf-8"},
				"Content-Length": {"9"},
			},
		},
		{
			name:           "Malformed Request",
			request:        "BADREQUEST\r\n\r\n",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "",
			expectedHeaders: http.Header{
				"Content-Length": {"0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Establish a TCP connection to the server
			conn, err := net.Dial("tcp", addr)
			require.NoError(t, err, "Failed to connect to server")
			defer conn.Close()

			// Send the HTTP request
			_, err = conn.Write([]byte(tt.request))
			require.NoError(t, err, "Failed to send request")

			// Read the response
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			responseBuf := new(bytes.Buffer)
			_, err = io.Copy(responseBuf, conn)
			if err != nil && err != io.EOF {
				t.Fatalf("Failed to read response: %v", err)
			}

			// Parse the response
			respReader := bufio.NewReader(bytes.NewReader(responseBuf.Bytes()))
			resp, err := http.ReadResponse(respReader, nil)
			require.NoError(t, err, "Failed to parse response")

			// Check status code
			assert.Equal(t, tt.expectedStatus, resp.StatusCode, "Unexpected status code")

			// Check headers
			for key, expectedValues := range tt.expectedHeaders {
				actualValues := resp.Header.Values(key)
				assert.Equal(t, expectedValues, actualValues, "Unexpected header values for %s", key)
			}

			// Read body
			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err, "Failed to read response body")
			resp.Body.Close()

			body := string(bodyBytes)

			// Check body
			assert.Equal(t, tt.expectedBody, body, "Unexpected response body")
		})
	}
}
