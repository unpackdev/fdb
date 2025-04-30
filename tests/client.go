package tests

import (
	"fmt"
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

	// Set read deadline based on timeout
	if err := client.SetReadDeadline(time.Now().Add(timeout)); err != nil {
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

	// Allocate buffer to read response
	// Using a reasonably sized buffer that should handle most responses
	responseBuf := make([]byte, 4096)

	// Read the response from the server
	n, err := client.Read(responseBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	latency := time.Since(start)

	t.logger.Debug("Received response",
		zap.Int("bytes", n),
		zap.String("from", client.RemoteAddr().String()),
		zap.Duration("latency", latency))

	// Return actual data read
	return responseBuf[:n], nil
}
