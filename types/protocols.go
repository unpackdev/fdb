// pkg/types/protocol.go
package types

import (
	"fmt"
)

// ProtocolType represents different application protocols.
type ProtocolType int

func (p ProtocolType) Uint32() uint32 {
	return uint32(p)
}

// ProtocolTypeFromUint32 converts a uint32 to a ProtocolType.
func ProtocolTypeFromUint32(u uint32) ProtocolType {
	switch u {
	case 0:
		return HTTPProtocol
	case 1:
		return RPCProtocol
	case 2:
		return WebSocketProtocol
	default:
		return -1 // Represents an unknown ProtocolType
	}
}

// ProtocolType constants using iota.
const (
	HTTPProtocol ProtocolType = iota
	RPCProtocol
	WebSocketProtocol
)

// String returns the string representation of the ProtocolType.
func (p ProtocolType) String() string {
	switch p {
	case HTTPProtocol:
		return "http"
	case RPCProtocol:
		return "rpc"
	case WebSocketProtocol:
		return "websocket"
	default:
		return "unknown"
	}
}

// ParseProtocolType parses a string into a ProtocolType.
func ParseProtocolType(s string) (ProtocolType, error) {
	switch s {
	case "http":
		return HTTPProtocol, nil
	case "rpc":
		return RPCProtocol, nil
	case "websocket":
		return WebSocketProtocol, nil
	default:
		return -1, fmt.Errorf("unknown protocol type: %s", s)
	}
}

// UnmarshalYAML allows ProtocolType to be correctly unmarshalled from a YAML string.
func (p *ProtocolType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	pt, err := ParseProtocolType(s)
	if err != nil {
		return err
	}

	*p = pt
	return nil
}
