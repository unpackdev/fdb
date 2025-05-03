// pkg/transports/tcp/test_helpers.go
package tcp

import (
	"context"
	"encoding/binary"

	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/observability"
	"github.com/unpackdev/fdb/pkg/types"

	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// CreateRequestMessage Helper function to create a length-prefixed message with ProtocolType (for requests).
func CreateRequestMessage(protocolType types.ProtocolType, payload []byte) []byte {
	// Prepend the protocol type byte
	messageWithProtocol := append([]byte{byte(protocolType)}, payload...)
	// Prefix with the length of the message
	length := uint32(len(messageWithProtocol))
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, length)
	return append(lengthBuf, messageWithProtocol...)
}

// CreateResponseMessage Helper function to create a length-prefixed message without ProtocolType (for responses).
func CreateResponseMessage(payload []byte) []byte {
	length := uint32(len(payload))
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, length)
	return append(lengthBuf, payload...)
}

// GetFreePortForTest attempts to find an available port and confirm that it's truly available.
func GetFreePortForTest(t testing.TB) int {
	maxAttempts := 5
	for i := 0; i < maxAttempts; i++ {
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		require.NoError(t, err, "Failed to resolve TCP address")

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			t.Logf("Attempt %d: Unable to acquire a free port - %v", i+1, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return port
	}

	t.Fatalf("Failed to acquire a free port after %d attempts", maxAttempts)
	return -1
}

// SetupServerTest sets up the TCP server with a dynamic port.
func SetupServerTest(t testing.TB, ctx context.Context, cfg config.TcpTransport) (logger.Logger, *observability.Observability, *Server) {
	nodeConfig := config.Config{
		Id: "test_node_1",
		Logger: config.Logger{
			Enabled:     true,
			Environment: "development",
			Level:       "error", // Set to debug for detailed logs
		},
		Observability: config.Observability{
			Metrics: config.MetricsConfig{
				Enable:         false,
				Exporter:       "prometheus",
				Endpoint:       "0.0.0.0:9090",
				Headers:        map[string]string{},
				ExportInterval: 15 * time.Second,
				SampleRate:     1.0,
			},
			Tracing: config.TracingConfig{
				Enable:         false,
				Exporter:       "otlp",
				Endpoint:       "localhost:4317",
				Headers:        map[string]string{},
				Sampler:        "always_on",
				SamplingRate:   1.0,
				ExportInterval: 15 * time.Second,
			},
		},
	}

	gLog, err := logger.InitializeGlobalLogger(nodeConfig.Id, nodeConfig.Logger)
	require.NoError(t, err, "Failed to initialize global logger")

	// Initialize Observability
	obs, err := observability.New(ctx, nodeConfig, gLog)
	require.NoError(t, err, "Failed to initialize Observability")
	require.NotNil(t, obs)

	// Create the server
	server, err := NewServer(ctx, cfg, gLog, obs)
	require.NoError(t, err, "Failed to initialize server")
	require.NotNil(t, server)

	return gLog, obs, server
}
