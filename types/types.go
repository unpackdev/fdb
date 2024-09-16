package types

import "fmt"

type TransportType int

// String representation of TransportType
func (t TransportType) String() string {
	switch t {
	case UDSTransportType:
		return "uds"
	case QUICTransportType:
		return "quic"
	case TCPTransportType:
		return "tcp"
	case UDPTransportType:
		return "udp"
	case DummyTransportType:
		return "dummy"
	default:
		return "unknown"
	}
}

// ParseTransportType parses a string into a TransportType
func ParseTransportType(s string) (TransportType, error) {
	switch s {
	case "uds":
		return UDSTransportType, nil
	case "quic":
		return QUICTransportType, nil
	case "tcp":
		return TCPTransportType, nil
	case "udp":
		return UDPTransportType, nil
	case "dummy":
		return DummyTransportType, nil
	default:
		return -1, fmt.Errorf("unknown transport type: %s", s)
	}
}

// UnmarshalYAML allows TransportType to be correctly unmarshalled from a YAML string
func (t *TransportType) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	tt, err := ParseTransportType(s)
	if err != nil {
		return err
	}

	*t = tt
	return nil
}

const (
	UDPTransportType TransportType = iota
	DummyTransportType
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
