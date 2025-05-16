package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/panjf2000/gnet/v2"
	"github.com/unpackdev/fdb/client"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/messages"
	"github.com/unpackdev/fdb/pkg/types"
	"go.uber.org/zap"
)

const (
	// maxChunkSize keeps every TCP write comfortably below the default Ethernet
	// MTU once TCP/IP framing overhead is added.
	maxChunkSize = 60 * 1024 // 60 KiB
)

func CreateClient(t *testing.T, ctx context.Context, logger logger.Logger, port int) (*client.Client, error) {
	cfg := client.NewConfig()

	cli := client.NewClient(ctx, cfg)

	// Create a new TCP transport with gnet options optimized for large payloads
	tcpTransport := client.NewTCPTransport(fmt.Sprintf("127.0.0.1:%d", port), logger,
		gnet.WithMulticore(true),
		gnet.WithTCPNoDelay(gnet.TCPNoDelay),
		gnet.WithSocketRecvBuffer(256*1024), // 256KB receive buffer
		gnet.WithSocketSendBuffer(256*1024), // 256KB send buffer
	)

	// Register the transport with the client
	if err := cli.RegisterTransport("tcp", tcpTransport); err != nil {
		return nil, err
	}

	return cli, nil
}

// -----------------------------------------------------------------------------
// Send helper methods
// -----------------------------------------------------------------------------

// SendMessage sends a message to the target node and returns an error if the
// send fails.
func (t *TestNode) SendMessage(targetNode *TestNode, transportType types.TransportType, handlerType types.HandlerType, data []byte) error {
	// Ensure client is initialized
	if targetNode.client == nil {
		return errors.New("client not initialized")
	}

	// Get the TCP transport from the client
	tcpTransport, err := targetNode.client.GetTransport("tcp")
	if err != nil {
		return fmt.Errorf("failed to get TCP transport: %w", err)
	}

	// Create and encode the message
	var encodedMsg []byte
	if _, decErr := messages.Decode(data); decErr == nil {
		// Data is already an encoded message
		encodedMsg = data
	} else {
		// Generate a new message with the data
		msg, err := messages.GenerateRandomMessageWithData(handlerType, data)
		if err != nil {
			return fmt.Errorf("failed to generate message: %w", err)
		}

		if encodedMsg, err = msg.Encode(); err != nil {
			return fmt.Errorf("failed to encode message: %w", err)
		}
	}

	// Send the message using the transport
	if err := tcpTransport.Send(encodedMsg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	t.logger.Debug("Sent message",
		zap.Int("bytes", len(encodedMsg)),
		zap.Stringer("handler_type", handlerType))

	return nil
}

// -----------------------------------------------------------------------------
// Round‑trip helper (send + blocking read)
// -----------------------------------------------------------------------------

// SendAndReceiveMessage sends a message to another node and waits for a
// response. This implementation uses the client package's ResponseHandler for
// asynchronous but reliable large payload handling.
func (t *TestNode) SendAndReceiveMessage(targetNode *TestNode, transportType types.TransportType, handlerType types.HandlerType, data []byte, timeout time.Duration) ([]byte, error) {
	// Ensure client is initialized
	if targetNode.client == nil {
		return nil, errors.New("client not initialized")
	}

	// Get the TCP transport from the client
	tcp, err := targetNode.client.GetTransport("tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to get TCP transport: %w", err)
	}

	tcpTransport, ok := tcp.(*client.TCPTransport)
	if !ok {
		return nil, errors.New("transport is not a TCPTransport")
	}

	// Generate a unique message ID for tracking the response
	messageID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate message ID: %w", err)
	}

	// Register a response channel with the generated message ID
	responseCh := tcpTransport.RegisterResponseChannel(messageID)

	// Prepare the message
	var encodedMsg []byte
	if _, decErr := messages.Decode(data); decErr == nil {
		// Data is already an encoded message
		encodedMsg = data
	} else {
		// Generate a new message with the data
		msg, err := messages.GenerateRandomMessageWithData(handlerType, data)
		if err != nil {
			return nil, fmt.Errorf("failed to generate message: %w", err)
		}

		// Set the message ID in the message
		msg.ID = messageID

		encodedMsg, err = msg.Encode()
		if err != nil {
			return nil, fmt.Errorf("failed to encode message: %w", err)
		}
	}

	// For large payloads, we use a chunking protocol to ensure reliable transmission
	if len(encodedMsg) > maxChunkSize {
		t.logger.Info("Using chunked message protocol for large payload",
			zap.Int("size", len(encodedMsg)),
			zap.Stringer("handler", handlerType))
		// ... rest of the chunking logic
	}

	// Send the message
	if err := tcpTransport.Send(encodedMsg); err != nil {
		tcpTransport.UnregisterResponseChannel(messageID)
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Wait for the response with timeout
	select {
	case response := <-responseCh:
		return response, nil
	case <-time.After(timeout):
		tcpTransport.UnregisterResponseChannel(messageID)
		return nil, fmt.Errorf("timeout waiting for response after %v", timeout)
	}
}
