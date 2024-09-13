package fdb

import (
	"bytes"
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
	buf := new(bytes.Buffer)

	// Write the HandlerType (1 byte)
	if err := buf.WriteByte(byte(m.Handler)); err != nil {
		return nil, fmt.Errorf("failed to encode handler: %w", err)
	}

	// Write the 32-byte key
	if _, err := buf.Write(m.Key[:]); err != nil {
		return nil, fmt.Errorf("failed to encode key: %w", err)
	}

	// Write the data (variable length)
	if _, err := buf.Write(m.Data); err != nil {
		return nil, fmt.Errorf("failed to encode data: %w", err)
	}

	return buf.Bytes(), nil
}

// Decode decodes a byte slice into a Message struct
func Decode(data []byte) (*Message, error) {
	if len(data) < 33 { // 1 byte for handler + 32 bytes for key
		return nil, fmt.Errorf("data too short, must be at least 33 bytes")
	}

	msg := &Message{}

	// Read the HandlerType (first byte)
	msg.Handler = HandlerType(data[0])

	// Read the 32-byte key
	copy(msg.Key[:], data[1:33])

	// Read the remaining data
	msg.Data = data[33:]

	return msg, nil
}
