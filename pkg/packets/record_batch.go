package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Record represents a key-value pair in a record batch
type Record struct {
	Key   [32]byte // Fixed-size byte array for keys
	Value []byte   // Value as byte slice
}

// RecordBatch represents a batch of records to be distributed
type RecordBatch struct {
	Records []Record
}

// Serialize converts a RecordBatch to a byte slice
func (rb *RecordBatch) Serialize() ([]byte, error) {
	var buffer bytes.Buffer

	// Write the number of records
	recordCount := uint32(len(rb.Records))
	if err := binary.Write(&buffer, binary.LittleEndian, recordCount); err != nil {
		return nil, fmt.Errorf("failed to write record count: %w", err)
	}

	// Write each record
	for _, record := range rb.Records {
		// Write the key (fixed 32 bytes)
		if _, err := buffer.Write(record.Key[:]); err != nil {
			return nil, fmt.Errorf("failed to write record key: %w", err)
		}

		// Write the value length
		valueLength := uint32(len(record.Value))
		if err := binary.Write(&buffer, binary.LittleEndian, valueLength); err != nil {
			return nil, fmt.Errorf("failed to write value length: %w", err)
		}

		// Write the value
		if _, err := buffer.Write(record.Value); err != nil {
			return nil, fmt.Errorf("failed to write record value: %w", err)
		}
	}

	return buffer.Bytes(), nil
}

// Deserialize parses a byte slice into a RecordBatch
func DeserializeRecordBatch(data []byte) (*RecordBatch, error) {
	buffer := bytes.NewReader(data)

	// Read record count
	var recordCount uint32
	if err := binary.Read(buffer, binary.LittleEndian, &recordCount); err != nil {
		return nil, fmt.Errorf("failed to read record count: %w", err)
	}

	// Create batch with capacity
	batch := &RecordBatch{
		Records: make([]Record, 0, recordCount),
	}

	// Read each record
	for i := uint32(0); i < recordCount; i++ {
		var record Record

		// Read key (fixed 32 bytes)
		if _, err := buffer.Read(record.Key[:]); err != nil {
			return nil, fmt.Errorf("failed to read record key: %w", err)
		}

		// Read value length
		var valueLength uint32
		if err := binary.Read(buffer, binary.LittleEndian, &valueLength); err != nil {
			return nil, fmt.Errorf("failed to read value length: %w", err)
		}

		// Read value - create a new slice for each record
		tempValue := make([]byte, valueLength)
		if _, err := buffer.Read(tempValue); err != nil {
			return nil, fmt.Errorf("failed to read record value: %w", err)
		}
		
		// Make a deep copy to avoid slice reference issues
		independentValue := make([]byte, valueLength)
		copy(independentValue, tempValue)
		
		// Create a completely new record with this value copy
		newRecord := Record{
			Key:   record.Key,   // Key is already a fixed-size array, so it's copied by value
			Value: independentValue, // Use our independent copy of the value
		}
		
		// Add the completely independent record to the batch
		batch.Records = append(batch.Records, newRecord)
	}

	return batch, nil
}
