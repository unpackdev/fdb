package fdb

import (
	"fmt"
)

// Message struct represents a UDP message
type Message struct {
	Handler HandlerType // The handler type (1 byte)
	Key     [32]byte    // Fixed 32-byte key (e.g., Ethereum hash)
	Data    []byte      // The remaining data after the key
}

// Encode encodes the Message struct into a byte slice
func (m *Message) Encode() ([]byte, error) {
	msgLen := 1 + 32 + len(m.Data)
	buf := make([]byte, msgLen)

	// Set handler type
	buf[0] = byte(m.Handler)

	// Copy the 32-byte key
	copy(buf[1:33], m.Key[:])

	// Copy the data
	copy(buf[33:], m.Data)

	return buf, nil
}

// Decode decodes a byte slice into a Message struct
func Decode(data []byte) (*Message, error) {
	if len(data) < 33 { // 1 byte for handler + 32 bytes for key
		return nil, fmt.Errorf("data too short, must be at least 33 bytes")
	}

	msg := &Message{
		Handler: HandlerType(data[0]),
	}

	// Copy the 32-byte key
	copy(msg.Key[:], data[1:33])

	// Assign the remaining data directly without copying
	msg.Data = data[33:]

	return msg, nil
}
