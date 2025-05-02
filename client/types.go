package client

import "errors"

type MessageType byte

func (t MessageType) Uint64() uint64 {
	return uint64(t)
}

var (
	InvalidActionMessageType MessageType = 0x69
	WriteSuccessMessageType  MessageType = 0x01
)

// Common error types for the client package
var (
	ErrResponseTimeout = errors.New("response timeout")
	ErrNoHandler       = errors.New("no handler for message type")
	ErrConnectionClosed = errors.New("connection closed")
)
