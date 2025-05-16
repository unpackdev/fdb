package packets

import (
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
	"github.com/unpackdev/fdb/pkg/types"
)

// MessageResponse represents a structured response from the database
// The wire format is: [status byte][16 bytes UUID][4 bytes length][data...]
type MessageResponse struct {
	Status types.HandlerStatus // 0x00 for error, 0x01 for success
	ID     uuid.UUID           // Correlation ID for request/response matching
	Length uint32              // Length of the data in bytes
	Data   []byte              // The actual response data
}

// IsSuccess returns true if the response status indicates success
func (r *MessageResponse) IsSuccess() bool {
	return r.Status == types.HandlerStatusSuccess
}

// IsError returns true if the response status indicates an error
func (r *MessageResponse) IsError() bool {
	return r.Status == types.HandlerStatusError
}

// GetErrorMessage returns the error message if Status is DBResponseStatusError
// If Status is not DBResponseStatusError, it returns an empty string
func (r *MessageResponse) GetErrorMessage() string {
	if !r.IsError() || len(r.Data) == 0 {
		return ""
	}
	return string(r.Data)
}

// Encode converts the MessageResponse to a byte slice for transmission
// Format: [status byte][16 bytes UUID][4 bytes length][data...]
func (r *MessageResponse) Encode() []byte {
	// 1 byte for status + 16 bytes for UUID + 4 bytes for length + data
	buffer := make([]byte, 1+16+4+len(r.Data))

	// Write status
	buffer[0] = r.Status.Byte()

	// Write UUID
	copy(buffer[1:17], r.ID[:])

	// Write length (big-endian)
	binary.BigEndian.PutUint32(buffer[17:21], r.Length)

	// Write data
	copy(buffer[21:], r.Data)

	return buffer
}

// DecodeMessageResponse parses a byte slice into a MessageResponse
// It expects the format: [status byte][16 bytes UUID][4 bytes length][data...]
func DecodeMessageResponse(data []byte) (*MessageResponse, error) {
	if len(data) < 21 { // Need at least status byte + 16 bytes UUID + 4 bytes for length
		return nil, errors.New("response data too short")
	}

	// Extract status (first byte)
	status := data[0]

	// Extract UUID (next 16 bytes)
	var id uuid.UUID
	copy(id[:], data[1:17])

	// Extract length (next 4 bytes in big-endian format)
	dataLength := binary.BigEndian.Uint32(data[17:21])

	// Extract data (everything after the status byte, UUID and length bytes)
	responseData := []byte{}
	if len(data) > 21 {
		responseData = data[21:]
	}

	// Handle case where responseData is longer than the specified length
	// This can happen if multiple responses are concatenated in the stream
	if uint32(len(responseData)) > dataLength {
		// Truncate the data to the specified length
		responseData = responseData[:dataLength]
	} else if uint32(len(responseData)) < dataLength {
		// Just use what we have when data is truncated - this happens with error messages
		// which can be truncated but still contain useful information
		dataLength = uint32(len(responseData))
	}

	return &MessageResponse{
		Status: types.HandlerStatus(status),
		ID:     id,
		Length: dataLength,
		Data:   responseData,
	}, nil
}
