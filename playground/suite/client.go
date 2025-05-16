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
	"github.com/google/uuid"
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

// SendMessage sends a message to the target node and returns an error if sending fails.
func (t *TestNode) SendMessage(ctx context.Context, targetNode *TestNode, transportType types.TransportType, handlerType types.HandlerType, data []byte) error {
	if targetNode.client == nil {
		return errors.New("client not initialized")
	}

	tcpTransport, err := targetNode.client.GetTransportByType(transportType)
	if err != nil {
		return fmt.Errorf("failed to get %s transport: %w", transportType, err)
	}

	var encodedMsg []byte
	if _, decErr := messages.Decode(data); decErr == nil { // Data is already an encoded message
		encodedMsg = data
	} else {
		msg, mErr := messages.GenerateRandomMessageWithData(handlerType, data)
		if mErr != nil {
			return fmt.Errorf("failed to generate message: %w", mErr)
		}

		if encodedMsg, err = msg.Encode(); err != nil {
			return fmt.Errorf("failed to encode message: %w", err)
		}
	}

	sendCh := make(chan error, 1)
	go func() {
		sendCh <- tcpTransport.Send(encodedMsg)
	}()

	select {
	case sErr := <-sendCh:
		if sErr != nil {
			return fmt.Errorf("failed to send message: %w", sErr)
		}
	case <-ctx.Done():
		return fmt.Errorf("message send cancelled: %w", ctx.Err())
	}

	t.logger.Debug(
		"Sent message",
		zap.Int("bytes", len(encodedMsg)),
		zap.Stringer("handler_type", handlerType),
	)

	return nil
}

// -----------------------------------------------------------------------------
// Round‑trip helper (send + blocking read)
// -----------------------------------------------------------------------------

// SendAndReceiveMessage sends a message to another node and waits for a
// response. This implementation uses the client package's ResponseHandler for
// asynchronous but reliable large payload handling.
func (t *TestNode) SendAndReceiveMessage(ctx context.Context, targetNode *TestNode, transportType types.TransportType, handlerType types.HandlerType, data []byte, timeout time.Duration) ([]byte, error) {
	if targetNode.client == nil {
		return nil, errors.New("client not initialized")
	}

	tcp, err := targetNode.client.GetTransportByType(transportType)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s transport: %w", transportType, err)
	}

	tcpTransport, ok := tcp.(*client.TCPTransport)
	if !ok {
		return nil, errors.New("transport is not a TCPTransport")
	}

	// Prepare the message
	var encodedMsg []byte
	var messageID uuid.UUID

	if decodedMsg, decErr := messages.Decode(data); decErr == nil {
		// Data is already an encoded message
		messageID = decodedMsg.ID
		encodedMsg = data
	} else {
		// Generate a new message with the data
		msg, err := messages.GenerateRandomMessageWithData(handlerType, data)
		if err != nil {
			return nil, fmt.Errorf("failed to generate message: %w", err)
		}

		messageID = msg.ID
		encodedMsg, err = msg.Encode()
		if err != nil {
			return nil, fmt.Errorf("failed to encode message: %w", err)
		}
	}

	// Register a response channel using the message ID for correlation
	responseCh := tcpTransport.RegisterResponseChannel(messageID)

	// For large payloads, we use a chunking protocol to ensure reliable transmission
	if len(encodedMsg) > maxChunkSize {
		t.logger.Info("Using chunked message protocol for large payload",
			zap.Int("size", len(encodedMsg)),
			zap.Stringer("handler", handlerType),
		)

		// Prepare the chunked message with a 4-byte length prefix
		// The server expects: [total_size(4 bytes)][payload...]
		lengthPrefix := make([]byte, 4)
		binary.LittleEndian.PutUint32(lengthPrefix, uint32(len(encodedMsg)))

		// Prepend the length prefix to the message
		chunkedMsg := append(lengthPrefix, encodedMsg...)

		// Replace the original message with the chunked version
		encodedMsg = chunkedMsg

		t.logger.Debug(
			"Prepared chunked message",
			zap.Int("original_size", len(encodedMsg)-4),
			zap.Int("with_prefix_size", len(encodedMsg)),
		)
	}

	// Send the message with context cancellation support
	start := time.Now()
	sendCh := make(chan error, 1)
	go func() {
		sendCh <- tcpTransport.Send(encodedMsg)
	}()

	// Wait for either the send to complete or the context to be cancelled
	select {
	case err := <-sendCh:
		if err != nil {
			// Make sure to unregister the response channel on error
			tcpTransport.UnregisterResponseChannel(messageID)
			return nil, fmt.Errorf("failed to send message: %w", err)
		}
	case <-ctx.Done():
		// Make sure to unregister the response channel on context cancellation
		tcpTransport.UnregisterResponseChannel(messageID)
		return nil, fmt.Errorf("message send cancelled: %w", ctx.Err())
	}

	t.logger.Debug(
		"Sent message",
		zap.Int("bytes", len(encodedMsg)),
		zap.Stringer("handler", handlerType),
		zap.String("message_id", messageID.String()),
	)

	// Create a response channel to get the result from WaitForResponseWithTimeout
	respCh := make(chan struct {
		resp []byte
		err  error
	}, 1)

	// Start a goroutine to wait for the response
	go func() {
		resp, err := tcpTransport.WaitForResponseWithTimeout(responseCh, timeout)
		respCh <- struct {
			resp []byte
			err  error
		}{resp, err}
	}()

	// Wait for either the response or context cancellation
	select {
	case result := <-respCh:
		if result.err != nil {
			return nil, result.err
		}
		latency := time.Since(start)
		t.logger.Debug(
			"Received response",
			zap.Int("bytes", len(result.resp)),
			zap.Duration("latency", latency),
		)
		return result.resp, nil
	case <-ctx.Done():
		// Context was cancelled while waiting for response
		tcpTransport.UnregisterResponseChannel(messageID)
		return nil, fmt.Errorf("waiting for response cancelled: %w", ctx.Err())
	}
}
