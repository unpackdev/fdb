package fdb

import "fmt"

type TransportType string

func (t TransportType) String() string {
	return string(t)
}

type DbType string

func (t DbType) String() string {
	return string(t)
}

// HandlerType represents different types of handlers
type HandlerType byte

// Define the handlers as 1-byte constants
const (
	WriteHandlerType HandlerType = 'W' // 'W' for WRITE
	ReadHandlerType  HandlerType = 'R' // 'R' for READ

	QuicTransportType TransportType = "QUIC"
)

// FromByte converts a byte into a HandlerType
func (h *HandlerType) FromByte(b byte) error {
	switch b {
	case 'W':
		*h = WriteHandlerType
	case 'R':
		*h = ReadHandlerType
	default:
		return fmt.Errorf("invalid action byte: %v", b)
	}
	return nil
}
