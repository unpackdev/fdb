package client_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/unpackdev/fdb/client"
	"github.com/unpackdev/fdb/messages"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
)

func TestTCPClientSendMessage(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	// Create configuration
	cfg := client.NewConfig()

	// Create a new client
	c := client.NewClient(ctx, cfg)

	// Create a new TCP transport with gnet options
	tcpTransport := client.NewTCPTransport("127.0.0.1:5011", logger,
		gnet.WithMulticore(true),
		gnet.WithTCPNoDelay(gnet.TCPNoDelay),
	)

	// Register the transport with the client
	err := c.RegisterTransport("tcp", tcpTransport)
	require.NoError(t, err, "Failed to register transport")

	// Start the client (connects all registered transports)
	err = c.Start(ctx)
	require.NoError(t, err, "failed to start client")

	// Define test cases for table-driven tests
	testCases := []struct {
		name          string
		messageType   client.MessageType
		expectedResp  string
		handlerType   types.HandlerType
		expectedError error
	}{
		{
			name:         "Valid Write Message",
			handlerType:  types.WriteHandlerType,
			messageType:  client.WriteSuccessMessageType,
			expectedResp: "",
		},
		/* Uncomment and add more test cases as needed */
		// {
		// 	name:         "Invalid Action Message",
		// 	messageType:  client.InvalidActionMessageType,
		// 	expectedResp: "Invalid Action Response",
		// 	handlerType:  client.InvalidActionMessageType,
		// },
		// {
		// 	name:         "Unknown Handler Message",
		// 	messageType:  0, // Assuming 0 is not mapped to any known handler
		// 	expectedResp: "Unknown Handler Response",
		// 	handlerType:  0,
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Start the client (connects all transports)
			err = c.Start(ctx)
			require.NoError(t, err, "Failed to start client")

			// Wait for connection to establish
			time.Sleep(1 * time.Second)

			// Generate a random message based on the message type in the test case
			msg, msgErr := messages.GenerateRandomMessage(tc.handlerType)
			assert.NoError(t, msgErr)
			require.NotNil(t, msg)

			// Encode the message
			encodedMsg, err := msg.Encode()
			require.NoError(t, err, "Failed to encode message")

			// Capture the time before sending the message
			sentTime := time.Now()

			// Register a handler for the expected response
			tcpTransport.RegisterHandler(tc.messageType, func(c gnet.Conn, data []byte) error {
				// Log the time when the response is received
				receivedTime := time.Now()
				duration := receivedTime.Sub(sentTime)

				t.Logf("Received response: %s", string(data))
				t.Logf("Time taken for response: %s", duration)

				// Check if the received response matches the expected one
				assert.Equal(t, tc.expectedResp, string(data))

				return nil
			})

			// Send the message
			err = tcpTransport.Send(encodedMsg)
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err, "Failed to send message")
			}

			// Wait for potential responses
			time.Sleep(2 * time.Second)

		})
	}

	// Close the client
	err = c.Close()
	require.NoError(t, err, "Failed to close all clients")
}
