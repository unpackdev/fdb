package tests

import (
	"bytes"
	"crypto/rand"
	"fmt"
)

// GenerateTestDataKB creates a byte slice of the exact size requested in KB.
// By default, it uses a repeating pattern for efficiency. Use randomData=true for
// cryptographically secure random data when testing hashing/encoding.
func GenerateTestDataKB(sizeKB int, randomData bool) ([]byte, error) {
	if sizeKB <= 0 {
		return nil, fmt.Errorf("size must be positive, got %d KB", sizeKB)
	}

	sizeBytes := sizeKB * 1024

	if randomData {
		// Generate cryptographically secure random data (slower)
		data := make([]byte, sizeBytes)
		_, err := rand.Read(data)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random data: %w", err)
		}
		return data, nil
	}

	// Generate pattern-based test data (faster)
	// For large sizes, we use a larger repeating pattern to avoid
	// patterns that might make compression algorithms too effective
	var pattern []byte
	if sizeKB < 10 {
		// Small data, simple pattern
		pattern = []byte{0x01, 0x02, 0x03, 0x04}
	} else if sizeKB < 100 {
		// Medium data, more complex pattern
		pattern = []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	} else {
		// Large data, more random-like pattern
		pattern = make([]byte, 16)
		_, _ = rand.Read(pattern) // Ignoring error as this is not security-critical
	}

	// Calculate repetitions needed to reach the desired size
	repetitions := sizeBytes / len(pattern)
	remainder := sizeBytes % len(pattern)

	data := bytes.Repeat(pattern, repetitions)
	if remainder > 0 {
		data = append(data, pattern[:remainder]...)
	}

	return data, nil
}

// GenerateTestDataMB creates a byte slice of the exact size requested in MB.
// A wrapper around GenerateTestDataKB for convenience.
func GenerateTestDataMB(sizeMB int, randomData bool) ([]byte, error) {
	return GenerateTestDataKB(sizeMB*1024, randomData)
}
