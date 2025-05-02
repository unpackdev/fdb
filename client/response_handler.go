package client

import (
	"sync"
	"time"

	"github.com/unpackdev/fdb/packets"
	"github.com/unpackdev/fdb/types"
)

// ResponseCallback defines a function that processes a response message
type ResponseCallback func(response *packets.DBResponse) error

// ResponseHandler manages both channel-based and callback-based responses
// for asynchronous message handling
type ResponseHandler struct {
	mu                sync.Mutex
	responseChans     map[MessageType]chan []byte      // Map message type to response channel
	responseCallbacks map[MessageType]ResponseCallback // Map message type to callback function
	defaultTimeout    time.Duration
}

// NewResponseHandler creates a new response handler with a default timeout
func NewResponseHandler(defaultTimeout time.Duration) *ResponseHandler {
	return &ResponseHandler{
		responseChans:     make(map[MessageType]chan []byte),
		responseCallbacks: make(map[MessageType]ResponseCallback),
		defaultTimeout:    defaultTimeout,
	}
}

// RegisterChannel registers a response channel for a specific message type
// and returns the channel
func (h *ResponseHandler) RegisterChannel(messageType MessageType) chan []byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If a channel already exists, close it to prevent resource leaks
	if ch, exists := h.responseChans[messageType]; exists {
		close(ch)
	}

	ch := make(chan []byte, 1) // Buffer of 1 to prevent blocking
	h.responseChans[messageType] = ch
	return ch
}

// RegisterCallback registers a callback function for a specific message type
func (h *ResponseHandler) RegisterCallback(messageType MessageType, callback ResponseCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.responseCallbacks[messageType] = callback
}

// UnregisterChannel removes a response channel for a specific message type
func (h *ResponseHandler) UnregisterChannel(messageType MessageType) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, exists := h.responseChans[messageType]; exists {
		close(ch)
		delete(h.responseChans, messageType)
	}
}

// UnregisterCallback removes a callback for a specific message type
func (h *ResponseHandler) UnregisterCallback(messageType MessageType) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.responseCallbacks, messageType)
}

// HandleResponse processes an incoming response and routes it to the appropriate handler
// It returns true if the response was handled, false otherwise
func (h *ResponseHandler) HandleResponse(messageType MessageType, data []byte) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	handled := false

	// Parse the response data into a DBResponse object
	dbResponse, err := packets.DecodeDBResponse(data)
	if err != nil {
		// If we can't parse the response, just use the raw data for channel-based handling
		// For callbacks, we'll need to handle the error separately
		dbResponse = nil
	}

	// Check for a channel and send the response
	if ch, exists := h.responseChans[messageType]; exists {
		// For channel-based handling, we still send the raw data for backward compatibility
		// Prepend the message type byte to preserve the complete message
		fullData := append([]byte{byte(messageType)}, data...)
		select {
		case ch <- fullData: // Try to send the complete response with type byte
			handled = true
		default: // Don't block if channel is full or closed
		}
		delete(h.responseChans, messageType) // Clean up after handling
	}

	// Check for a callback and execute it
	if callback, exists := h.responseCallbacks[messageType]; exists {
		go func(cb ResponseCallback, resp *packets.DBResponse, rawData []byte) {
			if resp == nil {
				// If we couldn't decode the response, create an error response
				resp = &packets.DBResponse{
					Status: types.HandlerStatusError,
					Length: uint32(len("Invalid response format")),
					Data:   []byte("Invalid response format"),
				}
			}

			// Execute the callback with the structured response
			if err := cb(resp); err != nil {
				// You might want to log this error in a real implementation
			}
		}(callback, dbResponse, data)
		delete(h.responseCallbacks, messageType) // Clean up after handling
		handled = true
	}

	return handled
}

// WaitForResponse waits for a response on the given channel with the default timeout
func (h *ResponseHandler) WaitForResponse(ch chan []byte) ([]byte, error) {
	return h.WaitForResponseWithTimeout(ch, h.defaultTimeout)
}

// WaitForResponseWithTimeout waits for a response on the given channel with a custom timeout
func (h *ResponseHandler) WaitForResponseWithTimeout(ch chan []byte, timeout time.Duration) ([]byte, error) {
	select {
	case response := <-ch:
		return response, nil
	case <-time.After(timeout):
		// Clean up the channel from our registry on timeout
		h.mu.Lock()
		defer h.mu.Unlock()

		for msgType, registeredCh := range h.responseChans {
			if registeredCh == ch {
				delete(h.responseChans, msgType)
				break
			}
		}

		return nil, ErrResponseTimeout
	}
}
