package messages

import (
	"encoding/binary"
	"fmt"
	"github.com/unpackdev/fdb/types"
)

// Message struct represents a UDP message
type Message struct {
	Handler types.HandlerType // The handler type (1 byte)
	Key     [32]byte          // Fixed 32-byte key (e.g., Ethereum hash)
	Data    []byte            // The remaining data after the key
}

// Encode encodes the Message struct into a byte slice
func (m *Message) Encode() ([]byte, error) {
	// Add 4 bytes for the length of the data
	msgLen := 1 + 32 + 4 + len(m.Data)
	buf := make([]byte, msgLen)

	// Set handler type
	buf[0] = byte(m.Handler)

	// Copy the 32-byte key
	copy(buf[1:33], m.Key[:])

	// Set the length of the data (4 bytes)
	binary.BigEndian.PutUint32(buf[33:37], uint32(len(m.Data)))

	// Copy the data
	copy(buf[37:], m.Data)

	return buf, nil
}

// Decode decodes a byte slice into a Message struct
func Decode(data []byte) (*Message, error) {
	if len(data) < 37 { // 1 byte for handler + 32 bytes for key + 4 bytes for data length
		return nil, fmt.Errorf("data too short, must be at least 37 bytes")
	}

	msg := &Message{
		Handler: types.HandlerType(data[0]),
	}

	// Copy the 32-byte key
	copy(msg.Key[:], data[1:33])

	// Read the 4-byte data length
	dataLen := binary.BigEndian.Uint32(data[33:37])

	// Ensure the length of the remaining data matches the declared length
	if len(data[37:]) < int(dataLen) {
		return nil, fmt.Errorf("data length mismatch, expected %d bytes but got %d bytes", dataLen, len(data[37:]))
	}

	// Copy the data
	msg.Data = data[37 : 37+dataLen]

	return msg, nil
}
