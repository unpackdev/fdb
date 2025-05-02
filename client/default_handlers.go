package client

import (
	"fmt"
	"strings"

	"github.com/panjf2000/gnet/v2"
	"github.com/unpackdev/fdb/packets"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
)

// RegisterDefaultHandlers registers default message type handlers for common error conditions
// and system messages that the client should always be able to handle.
func RegisterDefaultHandlers(transport *TCPTransport) {
	transport.logger.Info("Registering default message handlers for client")

	// Register handler for InvalidActionMessageType (69 decimal = 'E' ASCII)
	// This message type is sent by the server when it encounters issues with very large payloads
	// or other resource constraints
	transport.RegisterHandler(InvalidActionMessageType, handleInvalidAction)
	transport.logger.Info("Registered handler for InvalidActionMessageType",
		zap.Uint64("type", InvalidActionMessageType.Uint64()))

	// Register handler for HandlerStatusError (0) to handle formatted error responses
	errorMsgType := MessageType(types.HandlerStatusError.Byte())
	transport.RegisterHandler(errorMsgType, handleErrorResponse)
	transport.logger.Info("Registered handler for HandlerStatusError",
		zap.Uint64("type", errorMsgType.Uint64()))

	// Log the current handlers for debugging
	transport.mu.Lock()
	handlerTypes := make([]uint64, 0, len(transport.handlers))
	for k := range transport.handlers {
		handlerTypes = append(handlerTypes, k.Uint64())
	}
	transport.mu.Unlock()
	transport.logger.Info("Current registered message handlers", zap.Reflect("types", handlerTypes))
}

// handleInvalidAction processes InvalidActionMessageType (0x69) messages from the server
// These typically indicate resource constraints when processing large payloads
func handleInvalidAction(conn gnet.Conn, data []byte) error {
	// Try to parse the response data into a DBResponse object
	dbResponse, err := packets.DecodeDBResponse(data)
	if err != nil {
		handler, ok := conn.Context().(*tcpEventHandler)
		if ok && handler != nil && handler.transport != nil && handler.transport.logger != nil {
			handler.transport.logger.Error("Failed to decode InvalidAction response",
				zap.Error(err),
				zap.Int("data_length", len(data)))
		}
		return fmt.Errorf("failed to decode InvalidAction response: %w", err)
	}

	// Extract the error message if available
	errorMessage := string(dbResponse.Data)
	if errorMessage == "" {
		errorMessage = "Server resource limit exceeded or action invalid"
	}

	// Log the error
	handler, ok := conn.Context().(*tcpEventHandler)
	if ok && handler != nil && handler.transport != nil && handler.transport.logger != nil {
		handler.transport.logger.Warn("Server reported invalid action or resource constraint",
			zap.String("error", errorMessage),
			zap.Uint32("data_length", dbResponse.Length),
			zap.Uint8("status", uint8(dbResponse.Status)))
	}

	// Since this is a special error type, we can notify any waiting response channels
	// by converting this to a regular error response
	errorResp := &packets.DBResponse{
		Status: types.HandlerStatusError,
		Length: uint32(len(errorMessage)),
		Data:   []byte(errorMessage),
	}

	// Forward this error to any success message handlers that might be waiting
	if handler != nil && handler.transport != nil && handler.transport.responseHandler != nil {
		// Create a new message that response handlers can understand (success message type)
		successMsgType := MessageType(types.HandlerStatusSuccess.Byte())

		// Encode the error response to be handled
		encodedResp := errorResp.Encode()

		// Use the response handler's existing mechanism to handle this
		handler.transport.responseHandler.HandleResponse(successMsgType, encodedResp)
	}

	return nil
}

// handleErrorResponse processes HandlerStatusError (0) messages from the server
// These are properly formatted protocol error messages using DBResponse structure
func handleErrorResponse(conn gnet.Conn, data []byte) error {
	// Try to parse the response data into a DBResponse object
	dbResponse, err := packets.DecodeDBResponse(data)
	if err != nil {
		handler, ok := conn.Context().(*tcpEventHandler)
		if ok && handler != nil && handler.transport != nil && handler.transport.logger != nil {
			handler.transport.logger.Error("Failed to decode Error response",
				zap.Error(err),
				zap.Int("data_length", len(data)))
		}
		return fmt.Errorf("failed to decode Error response: %w", err)
	}

	// Extract the error message from the response
	errorMessage := string(dbResponse.Data)
	if errorMessage == "" {
		errorMessage = "Server reported error (no details available)"
	}
	
	// Clean up the message if multiple errors are concatenated
	// Look for first null byte or length marker which indicates concatenated messages
	if idx := strings.IndexByte(errorMessage, 0); idx > 0 {
		errorMessage = errorMessage[:idx]
	}
	
	// Extract the first complete error message if possible
	if strings.HasPrefix(errorMessage, "ERROR:") || strings.HasPrefix(errorMessage, "RROR:") {
		// Already have a clean error message
	} else if idx := strings.Index(errorMessage, "ERROR:"); idx >= 0 {
		errorMessage = errorMessage[idx:]
		// Check for end of this error message
		if endIdx := strings.IndexByte(errorMessage, 0); endIdx > 0 {
			errorMessage = errorMessage[:endIdx]
		}
	}

	// Log the error
	handler, ok := conn.Context().(*tcpEventHandler)
	if ok && handler != nil && handler.transport != nil && handler.transport.logger != nil {
		handler.transport.logger.Error("Received protocol-formatted error from server",
			zap.String("error_message", errorMessage),
			zap.Uint32("data_length", dbResponse.Length))
	}

	// Forward this error to any success message handlers that might be waiting
	if handler != nil && handler.transport != nil && handler.transport.responseHandler != nil {
		// Create a new message that response handlers can understand (success message type)
		successMsgType := MessageType(types.HandlerStatusSuccess.Byte())

		// Just pass the original data - it's already properly formatted
		handler.transport.responseHandler.HandleResponse(successMsgType, data)
	}

	return nil
}
