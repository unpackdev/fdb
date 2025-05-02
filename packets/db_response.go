package packets

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/unpackdev/fdb/types"
)

// Define handler status constants directly to avoid circular imports
const (
	DBResponseStatusError   byte = 0x00 // 0 in decimal
	DBResponseStatusSuccess byte = 0x01 // 1 in decimal
)

// DBResponse represents a structured response from the database
// The wire format is: [status byte][4 bytes length][data...]
type DBResponse struct {
	Status types.HandlerStatus // 0x00 for error, 0x01 for success
	Length uint32              // Length of the data in bytes
	Data   []byte              // The actual response data
}

// IsSuccess returns true if the response status indicates success
func (r *DBResponse) IsSuccess() bool {
	return r.Status == types.HandlerStatusSuccess
}

// IsError returns true if the response status indicates an error
func (r *DBResponse) IsError() bool {
	return r.Status == types.HandlerStatusError
}

// GetErrorMessage returns the error message if Status is DBResponseStatusError
// If Status is not DBResponseStatusError, it returns an empty string
func (r *DBResponse) GetErrorMessage() string {
	if !r.IsError() || len(r.Data) == 0 {
		return ""
	}
	return string(r.Data)
}

// Encode converts the DBResponse to a byte slice for transmission
// Format: [status byte][4 bytes length][data...]
func (r *DBResponse) Encode() []byte {
	// 1 byte for status + 4 bytes for length + data
	buffer := make([]byte, 1+4+len(r.Data))

	// Write status
	buffer[0] = r.Status.Byte()

	// Write length (big-endian)
	binary.BigEndian.PutUint32(buffer[1:5], r.Length)

	// Write data
	copy(buffer[5:], r.Data)

	return buffer
}

// DecodeDBResponse parses a byte slice into a DBResponse
// It expects the format: [status byte][4 bytes length][data...]
func DecodeDBResponse(data []byte) (*DBResponse, error) {
	if len(data) < 5 { // Need at least status byte + 4 bytes for length
		return nil, errors.New("response data too short")
	}

	// Extract status (first byte)
	status := data[0]

	// Extract length (next 4 bytes in big-endian format)
	dataLength := binary.BigEndian.Uint32(data[1:5])

	// Extract data (everything after the status byte and length bytes)
	responseData := []byte{}
	if len(data) > 5 {
		responseData = data[5:]
	}

	// Handle case where responseData is longer than the specified length
	// This can happen if multiple responses are concatenated in the stream
	if uint32(len(responseData)) > dataLength {
		// Truncate the data to the specified length
		responseData = responseData[:dataLength]
	} else if uint32(len(responseData)) < dataLength {
		// Only consider it an error if we have less data than expected
		return nil, errors.New("response data truncated: expected " + fmt.Sprintf("%d", dataLength) + " bytes but got " + fmt.Sprintf("%d", len(responseData)))
	}

	return &DBResponse{
		Status: types.HandlerStatus(status),
		Length: dataLength,
		Data:   responseData,
	}, nil
}
