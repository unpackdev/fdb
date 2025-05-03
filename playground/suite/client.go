package suite

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

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

func CreateClient(ctx context.Context, logger logger.Logger, port int) (*client.Client, error) {
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

	// Determine the appropriate message type for the response
	responseType := client.MessageType(types.HandlerStatusSuccess.Byte())

	// Register a response channel before sending the message
	responseCh := tcpTransport.RegisterResponseChannel(responseType)

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

		// Prepare the chunked message with a 4-byte length prefix
		// The server expects: [total_size(4 bytes)][payload...]
		lengthPrefix := make([]byte, 4)
		binary.LittleEndian.PutUint32(lengthPrefix, uint32(len(encodedMsg)))

		// Prepend the length prefix to the message
		chunkedMsg := append(lengthPrefix, encodedMsg...)

		// Replace the original message with the chunked version
		encodedMsg = chunkedMsg

		t.logger.Debug("Prepared chunked message",
			zap.Int("original_size", len(encodedMsg)-4),
			zap.Int("with_prefix_size", len(encodedMsg)))
	}

	// Send the message
	start := time.Now()
	if err := tcpTransport.Send(encodedMsg); err != nil {
		// Make sure to unregister the response channel on error
		tcpTransport.UnregisterResponseChannel(responseType)
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	t.logger.Debug("Sent message",
		zap.Int("bytes", len(encodedMsg)),
		zap.Stringer("handler", handlerType))

	// Wait for the response with the provided timeout
	response, err := tcpTransport.WaitForResponseWithTimeout(responseCh, timeout)
	if err != nil {
		return nil, fmt.Errorf("error waiting for response: %w", err)
	}

	latency := time.Since(start)
	t.logger.Debug("Received response",
		zap.Int("bytes", len(response)),
		zap.Duration("latency", latency))
	return response, nil
}
