package types

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashFunctions(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectedHex string
	}{
		{
			name:        "Hash of empty data",
			input:       []byte{},
			expectedHex: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:        "Hash of simple data",
			input:       []byte("hello world"),
			expectedHex: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:        "Hash of numeric data",
			input:       []byte{0x01, 0x02, 0x03},
			expectedHex: "039058c6f2c0cb492c533b0a4d14ef77cc0f78abccced5287d84a1a2011cfb81",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := SumHash(tt.input)
			hexStr := h.Hex()
			assert.Equal(t, tt.expectedHex, hexStr, "Hash of input data should match expected value")
		})
	}
}

func TestHashFromBytes(t *testing.T) {
	validBytes := make([]byte, HashSize)
	for i := 0; i < HashSize; i++ {
		validBytes[i] = byte(i)
	}
	invalidBytes := make([]byte, HashSize-1)

	t.Run("HashFromBytes with valid bytes", func(t *testing.T) {
		h, err := HashFromBytes(validBytes)
		require.NoError(t, err)
		assert.Equal(t, validBytes, h.Bytes())
	})

	t.Run("HashFromBytes with invalid bytes", func(t *testing.T) {
		_, err := HashFromBytes(invalidBytes)
		assert.Error(t, err)
	})
}

func TestHashEquality(t *testing.T) {
	hash1 := SumHash([]byte("test"))
	hash2 := SumHash([]byte("test"))
	hash3 := SumHash([]byte("different"))

	assert.True(t, HashEqual(hash1, hash2), "Hashes of identical data should be equal")
	assert.False(t, HashEqual(hash1, hash3), "Hashes of different data should not be equal")
}

func TestZeroHash(t *testing.T) {
	h := Hash{}
	assert.True(t, IsZeroHash(h), "Empty hash should be recognized as zero hash")

	h = SumHash([]byte("data"))
	assert.False(t, IsZeroHash(h), "Non-empty hash should not be recognized as zero hash")
}

func TestHashMarshalling(t *testing.T) {
	h := SumHash([]byte("test data"))

	// Marshal to text
	text, err := h.MarshalText()
	require.NoError(t, err)
	expectedText := hex.EncodeToString(h[:])
	assert.Equal(t, expectedText, string(text), "Marshalled text should match expected value")

	// Unmarshal from text
	var newHash Hash
	err = newHash.UnmarshalText(text)
	require.NoError(t, err)
	assert.Equal(t, h, newHash, "Unmarshalled hash should match original")

	// Marshal to binary
	binaryData, err := h.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, h.Bytes(), binaryData, "Marshalled binary should match byte representation")

	// Unmarshal from binary
	err = newHash.UnmarshalBinary(binaryData)
	require.NoError(t, err)
	assert.Equal(t, h, newHash, "Unmarshalled binary hash should match original")
}
