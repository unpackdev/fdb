package websocket

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"
	"github.com/unpackdev/fdb/pkg/transports/tcp"
	"github.com/unpackdev/fdb/pkg/types"
)

func setupWebSocketServerTest(t testing.TB, ctx context.Context) (*tcp.Server, logger.Logger, *observability.Observability, string) {
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

	// Initialize WebSocket Handler and Register
	wsHandler := NewTrafficHandler(gLog)
	server.SetOnTrafficHandler(wsHandler.Handle)

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
func TestWebSocketHandler(t *testing.T) {
	// Create a context and setup WebSocket server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, _, _, addr := setupWebSocketServerTest(t, ctx)
	defer func() {
		if server != nil {
			if err := server.Stop(); err != nil {
				t.Fatalf("Failed to stop server: %v", err)
			}
		}
	}()

	u := url.URL{Scheme: "ws", Host: addr, Path: "/"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err, "Failed to connect to WebSocket server")
	defer c.Close()

	closeCh := make(chan struct{})

	// Start a goroutine to read messages from the WebSocket server
	go func() {
		defer close(closeCh)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				t.Logf("Read error: %v", err)
				return
			}
			t.Logf("Received: %s", message)

			return
		}
	}()

	message := "Hello, WebSocket!"
	err = c.WriteMessage(websocket.TextMessage, []byte(message))
	require.NoError(t, err, "Failed to send WebSocket text message")

	select {
	case <-closeCh:
		return
	case <-time.After(5 * time.Second):
		t.Fatalf("WebSocket handler did not respond in time")
	}
}
