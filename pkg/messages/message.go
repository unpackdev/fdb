package messages

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/unpackdev/fdb/pkg/types"
)

// Message struct represents a UDP message
type Message struct {
	Handler  types.HandlerType // The handler type (1 byte)
	Priority types.Priority    // The priority (1 byte)
	ID       uuid.UUID         // Message identifier for correlation (16 bytes)
	Key      [32]byte          // Fixed 32-byte key (e.g., Ethereum hash)
	Data     []byte            // The remaining data after the key
}

// EncodeWithBuffer encodes the Message struct into a provided byte slice (buffer).
// Assumes the buffer is large enough and avoids allocating new buffers.
// Designed to be used with sync.Pool
func (m *Message) EncodeWithBuffer(buf []byte) ([]byte, error) {
	// Calculate the total message length (1 byte for handler + 1 byte for priority + 16 bytes for UUID + 32 bytes for key + 4 bytes for data length + actual data)
	msgLen := 1 + 1 + 16 + 32 + 4 + len(m.Data)

	// Ensure the buffer is large enough (zero-allocation requires that the buffer be managed externally)
	if len(buf) < msgLen {
		return nil, fmt.Errorf("buffer too small, need at least %d bytes", msgLen)
	}

	// Set handler type
	buf[0] = byte(m.Handler)

	// Set priority
	buf[1] = byte(m.Priority)

	// Copy UUID
	copy(buf[2:18], m.ID[:])

	// Copy the 32-byte key
	copy(buf[18:50], m.Key[:])

	// Set the length of the data (4 bytes)
	binary.BigEndian.PutUint32(buf[50:54], uint32(len(m.Data)))

	// Copy the data
	copy(buf[54:], m.Data)

	// Return the portion of the buffer that was actually used
	return buf[:msgLen], nil
}

// Encode encodes the Message struct into a byte slice.
// This method allocates a new buffer for every call, unlike EncodeWithBuffer which reuses a buffer.
func (m *Message) Encode() ([]byte, error) {
	// Calculate the total message length (1 byte for handler + 1 byte for priority + 16 bytes for UUID + 32 bytes for key + 4 bytes for data length + actual data)
	msgLen := 1 + 1 + 16 + 32 + 4 + len(m.Data)

	// Allocate a new buffer
	buf := make([]byte, msgLen)

	// Set handler type
	buf[0] = byte(m.Handler)

	// Set priority
	buf[1] = byte(m.Priority)

	// Copy UUID
	copy(buf[2:18], m.ID[:])

	// Copy the 32-byte key
	copy(buf[18:50], m.Key[:])

	// Set the length of the data (4 bytes)
	binary.BigEndian.PutUint32(buf[50:54], uint32(len(m.Data)))

	// Copy the data
	copy(buf[54:], m.Data)

	return buf, nil
}

// Decode decodes a byte slice into a Message struct without allocating new memory for data.
func Decode(data []byte) (*Message, error) {
	if len(data) < 54 { // 1 byte for handler + 1 byte for priority + 16 bytes for UUID + 32 bytes for key + 4 bytes for data length
		return nil, fmt.Errorf("data too short, must be at least 54 bytes")
	}

	msg := &Message{
		Handler:  types.HandlerType(data[0]),
		Priority: types.Priority(data[1]),
	}

	// Copy the UUID
	copy(msg.ID[:], data[2:18])

	// Copy the 32-byte key
	copy(msg.Key[:], data[18:50])

	// Read the 4-byte data length
	dataLen := binary.BigEndian.Uint32(data[50:54])

	// Ensure the length of the remaining data matches the declared length
	if len(data[54:]) < int(dataLen) {
		return nil, fmt.Errorf("data length mismatch, expected %d bytes but got %d bytes", dataLen, len(data[54:]))
	}

	// Reuse the data slice instead of allocating a new one
	msg.Data = data[54 : 54+dataLen]

	return msg, nil
}

// GenerateRandomMessage generates a Message with a random handler and key, and no data.
func GenerateRandomMessage(handler types.HandlerType) (*Message, error) {
	key, err := GenerateRandomKey()
	if err != nil {
		return nil, err
	}

	return &Message{
		Handler:  handler,
		Priority: types.PriorityNormal, // Default to normal priority
		ID:       uuid.New(),           // Generate a new UUID
		Key:      key,
		Data:     nil, // No data for this message
	}, nil
}

// GenerateRandomMessageWithData generates a Message with a key, and a specified data payload.
func GenerateRandomMessageWithData(handler types.HandlerType, data []byte) (*Message, error) {
	key, err := GenerateRandomKey()
	if err != nil {
		return nil, err
	}

	return &Message{
		Handler:  handler,
		Priority: types.PriorityNormal, // Default to normal priority
		ID:       uuid.New(),           // Generate a new UUID
		Key:      key,
		Data:     data,
	}, nil
}

// GenerateRandomKey Helper function to generate a random 32-byte key.
func GenerateRandomKey() ([32]byte, error) {
	var key [32]byte
	_, err := io.ReadFull(rand.Reader, key[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}
