package tests

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/messages"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
)

// AcquireClient creates and returns a new TCP client connection to the target node
func (t *TestNode) AcquireClient(targetNode *TestNode) (*net.TCPConn, error) {
	// Get the target node's TCP address - need to find the correct TCP transport config
	var tcpAddress string
	for _, transport := range targetNode.Config().Transports {
		if transport.Type == types.TCPTransportType && transport.Enabled {
			// Extract the TCP configuration
			tcpConfig, ok := transport.Config.(*config.TcpTransport)
			if !ok || !tcpConfig.Enabled {
				continue
			}

			// Create the address string
			tcpAddress = fmt.Sprintf("%s:%d", tcpConfig.IPv4, tcpConfig.Port)
			break
		}
	}

	if tcpAddress == "" {
		return nil, fmt.Errorf("no TCP transport configured for target node")
	}

	// Resolve the server address
	serverAddr, err := net.ResolveTCPAddr("tcp", tcpAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve target address %s: %w", tcpAddress, err)
	}

	// Create the TCP client
	client, err := net.DialTCP("tcp", nil, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to target node %s: %w", tcpAddress, err)
	}

	t.logger.Debug("Acquired TCP client connection", zap.String("to", tcpAddress))
	return client, nil
}

// SendMessage sends a message to the target node and returns an error if the send fails
func (t *TestNode) SendMessage(targetNode *TestNode, transportType types.TransportType, handlerType types.HandlerType, data []byte) error {
	// Acquire a direct TCP connection to the target node
	client, err := t.AcquireClient(targetNode)
	if err != nil {
		return err
	}
	defer client.Close()

	// Generate a message with the provided data
	msg, err := messages.GenerateRandomMessageWithData(handlerType, data)
	if err != nil {
		return fmt.Errorf("failed to generate message: %w", err)
	}

	// Encode the message
	encodedMsg, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Write the message to the server
	_, err = client.Write(encodedMsg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	t.logger.Debug("Sent message",
		zap.Int("bytes", len(encodedMsg)),
		zap.String("to", client.RemoteAddr().String()))

	return nil
}

// SendAndReceiveMessage sends a message to another node and directly waits for a response
// Similar to the benchmark code, this handles the connection directly
func (t *TestNode) SendAndReceiveMessage(targetNode *TestNode, transportType types.TransportType,
	handlerType types.HandlerType, data []byte, timeout time.Duration) ([]byte, error) {
	// Acquire a direct TCP connection to the target node
	client, err := t.AcquireClient(targetNode)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var encodedMsg []byte

	// Check if data is already a properly encoded Message
	// Try to decode it as a Message to see if it's valid
	_, decodeErr := messages.Decode(data)
	if decodeErr == nil {
		// Data is already a properly encoded Message
		encodedMsg = data
	} else {
		// Data is not a properly encoded Message, so generate a new one with random key
		msg, err := messages.GenerateRandomMessageWithData(handlerType, data)
		if err != nil {
			return nil, fmt.Errorf("failed to generate message: %w", err)
		}

		// Encode the message
		encodedMsg, err = msg.Encode()
		if err != nil {
			return nil, fmt.Errorf("failed to encode message: %w", err)
		}
	}

	// Set read deadline based on timeout - but use at least 5 seconds for large payloads
	readTimeout := timeout
	if readTimeout < 5*time.Second {
		readTimeout = 5 * time.Second
	}
	if err := client.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Start measuring latency
	start := time.Now()

	// Write the message to the server
	_, err = client.Write(encodedMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	t.logger.Debug("Sent message",
		zap.Int("bytes", len(encodedMsg)),
		zap.String("to", client.RemoteAddr().String()))

	// Allocate a larger buffer to read responses - big enough for large payloads
	// 256KB buffer ensures we can handle even larger payloads than 65KB
	bufSize := 256 * 1024 
	buf := make([]byte, bufSize) 
	var responseBuf bytes.Buffer

	// Use a generous overall timeout for the entire operation
	baseTimeout := 5 * time.Second
	if timeout > baseTimeout {
		baseTimeout = timeout
	}
	
	// Deadline for the entire operation 
	operationDeadline := time.Now().Add(baseTimeout)
	
	// Set overall deadline
	if err := client.SetDeadline(operationDeadline); err != nil {
		return nil, fmt.Errorf("failed to set operation deadline: %w", err)
	}

	// Track our reading progress
	totalBytesRead := 0
	lastReadSize := 0
	noProgressCount := 0
	
	// Read in a loop until we get EOF or timeout
	for {
		// Update how much time we have left for this read
		timeRemaining := operationDeadline.Sub(time.Now())
		if timeRemaining <= 0 {
			break // Total operation timeout reached
		}
		
		// Set timeout for this specific read operation
		readTimeout := 500 * time.Millisecond
		if timeRemaining < readTimeout {
			readTimeout = timeRemaining
		}
		client.SetReadDeadline(time.Now().Add(readTimeout))

		// Try reading into our buffer
		n, err := client.Read(buf)
		
		// Check for progress (debugging for stuck reads)
		if n == lastReadSize {
			noProgressCount++
			if noProgressCount > 5 {
				t.logger.Debug("No new data after multiple reads", 
					zap.Int("total_received", totalBytesRead))
				break // Assume we're done if we keep getting the same amount of data
			}
		} else {
			noProgressCount = 0
			lastReadSize = n
		}

		// If we read something, add it to our result buffer
		if n > 0 {
			totalBytesRead += n
			responseBuf.Write(buf[:n])
			t.logger.Debug("Read chunk of data", 
				zap.Int("chunk_bytes", n), 
				zap.Int("total_bytes", totalBytesRead))
		}

		// Handle different read outcomes
		if err != nil {
			// EOF means we reached the end of data cleanly
			if errors.Is(err, io.EOF) {
				t.logger.Debug("Received EOF", zap.Int("total_bytes", totalBytesRead))
				break
			}

			// Handle timeout - could mean we're done or server is slow
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// If we have some data but hit a timeout, we might be done
				if totalBytesRead > 0 {
					t.logger.Debug("Read timeout with data", zap.Int("total_bytes", totalBytesRead))
					
					// If we've read more than 64KB, we're likely done with large payloads
					// This helps with boundary cases when the server doesn't properly signal EOF
					if totalBytesRead > 65*1024 {
						break
					}
					
					// Otherwise, try another read
					continue
				} else {
					// No data at all means server may be unresponsive
					return nil, fmt.Errorf("timeout waiting for server response")
				}
			}

			// For any other error, return it
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		
		// If we filled our read buffer completely, continue immediately to read more
		// Otherwise, a short read with no error might mean we're at the end
		if n < bufSize {
			// Short read with no error - means we got all available data
			t.logger.Debug("Short read with no error, likely complete", 
				zap.Int("read_size", n), 
				zap.Int("buffer_size", bufSize))
			break
		}
	}

	latency := time.Since(start)

	t.logger.Debug("Received response",
		zap.Int("bytes", responseBuf.Len()),
		zap.String("from", client.RemoteAddr().String()),
		zap.Duration("latency", latency))

	// Return the complete data from the buffer
	return responseBuf.Bytes(), nil
}
