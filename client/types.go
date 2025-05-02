package client

import "errors"

type MessageType byte

func (t MessageType) Uint64() uint64 {
	return uint64(t)
}

var (
	// InvalidActionMessageType is 69 decimal (not 0x69 hex which would be 105 decimal)
	InvalidActionMessageType MessageType = 69 // 'E' in ASCII
	WriteSuccessMessageType  MessageType = 0x01
)

// Common error types for the client package
var (
	ErrResponseTimeout = errors.New("response timeout")
	ErrNoHandler       = errors.New("no handler for message type")
	ErrConnectionClosed = errors.New("connection closed")
)
