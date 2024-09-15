package types

import "fmt"

type TransportType int

const (
	UDPTransportType TransportType = iota
	QUICTransportType
	UDSTransportType
	TCPTransportType
)

type DbType string

func (t DbType) String() string {
	return string(t)
}

const (
// To be defined for database types in the future...
)

// HandlerType represents different types of handlers
type HandlerType byte

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

// Define the handlers as 1-byte constants
const (
	WriteHandlerType HandlerType = 'W' // 'W' for WRITE
	ReadHandlerType  HandlerType = 'R' // 'R' for READ
)
