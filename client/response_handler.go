package client

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/unpackdev/fdb/pkg/packets"
)

// ResponseCallback defines a function that processes a response message
type ResponseCallback func(response *packets.MessageResponse) error

// ResponseHandler manages both channel-based and callback-based responses
// for asynchronous message handling
type ResponseHandler struct {
	mu                sync.Mutex
	responseChans     map[uuid.UUID]chan []byte      // Map message UUID to response channel
	responseCallbacks map[uuid.UUID]ResponseCallback // Map message UUID to callback function
	defaultTimeout    time.Duration
}

// NewResponseHandler creates a new response handler with a default timeout
func NewResponseHandler(defaultTimeout time.Duration) *ResponseHandler {
	return &ResponseHandler{
		responseChans:     make(map[uuid.UUID]chan []byte),
		responseCallbacks: make(map[uuid.UUID]ResponseCallback),
		defaultTimeout:    defaultTimeout,
	}
}

// RegisterChannel registers a response channel for a specific message ID
// and returns the channel
func (h *ResponseHandler) RegisterChannel(messageID uuid.UUID) chan []byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If a channel already exists, close it to prevent resource leaks
	if ch, exists := h.responseChans[messageID]; exists {
		close(ch)
	}

	ch := make(chan []byte, 1) // Buffer of 1 to prevent blocking
	h.responseChans[messageID] = ch
	return ch
}

// RegisterCallback registers a callback function for a specific message ID
func (h *ResponseHandler) RegisterCallback(messageID uuid.UUID, callback ResponseCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.responseCallbacks[messageID] = callback
}

// UnregisterChannel removes a response channel for a specific message ID
func (h *ResponseHandler) UnregisterChannel(messageID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, exists := h.responseChans[messageID]; exists {
		close(ch)
		delete(h.responseChans, messageID)
	}
}

// UnregisterCallback removes a callback for a specific message ID
func (h *ResponseHandler) UnregisterCallback(messageID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.responseCallbacks, messageID)
}

// HandleResponse processes an incoming response and routes it to the appropriate handler
// It returns true if the response was handled, false otherwise
func (h *ResponseHandler) HandleResponse(messageType MessageType, data []byte) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	handled := false

	// Parse the response data into a MessageResponse object
	response, err := packets.DecodeMessageResponse(data)
	if err != nil {
		// If we can't parse the response, we can't route it by UUID
		return false
	}

	// Check for a channel and send the response
	if ch, exists := h.responseChans[response.ID]; exists {
		// For channel-based handling, we still send the raw data
		// Prepend the message type byte to preserve the complete message
		fullData := append([]byte{byte(messageType)}, data...)
		select {
		case ch <- fullData: // Try to send the complete response with type byte
			handled = true
		default: // Don't block if channel is full or closed
		}
		delete(h.responseChans, response.ID) // Clean up after handling
	}

	// Check for a callback and execute it
	if callback, exists := h.responseCallbacks[response.ID]; exists {
		go func(cb ResponseCallback, resp *packets.MessageResponse) {
			// Execute the callback with the structured response
			if err := cb(resp); err != nil {
				// You might want to log this error in a real implementation
			}
		}(callback, response)
		delete(h.responseCallbacks, response.ID) // Clean up after handling
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

		for msgID, registeredCh := range h.responseChans {
			if registeredCh == ch {
				delete(h.responseChans, msgID)
				break
			}
		}

		return nil, ErrResponseTimeout
	}
}
