package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"strings"
)

var (
	ZeroHash = Hash{}

	ErrInvalidHashLength = errors.New("invalid address length")
)

// Hash represents a 32-byte SHA-256 hash.
type Hash [HashSize]byte

// SumHash computes the SHA-256 hash of the input data.
func SumHash(data []byte) Hash {
	return sha256.Sum256(data)
}

// HashEqual compares two hashes for equality.
func HashEqual(a, b Hash) bool {
	return bytes.Equal(a[:], b[:])
}

// HashData creates a SHA-256 hash of the input data.
func HashData(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func IsZeroHash(h Hash) bool {
	zeroHash := make([]byte, len(h))
	return bytes.Equal(h[:], zeroHash)
}

// HashFromBytes creates a Hash from a byte slice.
// Returns an error if the slice is not exactly HashSize bytes.
func HashFromBytes(data []byte) (Hash, error) {
	var h Hash
	if len(data) != HashSize {
		return h, fmt.Errorf("invalid hash length: expected %d bytes, got %d", HashSize, len(data))
	}
	copy(h[:], data)
	return h, nil
}

// FromCommonHash converts common.Hash to types.Hash.
func FromCommonHash(cAddr common.Hash) Hash {
	return Hash(cAddr)
}

// HashFromHex creates an Hash from a hex string.
// The hex string may have a "0x" prefix.
func HashFromHex(s string) (Hash, error) {
	var hash Hash
	cleaned := strings.TrimPrefix(s, "0x")
	if len(cleaned) != HashSize*2 {
		return hash, ErrInvalidHashLength
	}

	hBytes, err := hex.DecodeString(cleaned)
	if err != nil {
		return hash, err
	}
	copy(hash[:], hBytes)
	return hash, nil
}

// Bytes returns the byte slice representation of the Hash.
func (h Hash) Bytes() []byte {
	return h[:]
}

// String returns the hexadecimal string representation of the Hash.
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// Hex returns the hexadecimal string representation of the Hash.
func (h Hash) Hex() string {
	return "0x" + hex.EncodeToString(h[:])
}

// Equal compares two Hashes for equality.
func (h Hash) Equal(other Hash) bool {
	return h == other
}

// MarshalText implements the encoding.TextMarshaler interface.
func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.Hex()), nil
}

func (h Hash) ToCommonHash() common.Hash {
	return common.BytesToHash(h[:])
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (h *Hash) UnmarshalText(text []byte) error {
	cleaned := strings.TrimPrefix(string(text), "0x")
	bytes, err := hex.DecodeString(cleaned)
	if err != nil {
		return err
	}
	if len(bytes) != HashSize {
		return fmt.Errorf("invalid hash length: expected %d bytes, got %d", HashSize, len(bytes))
	}
	copy(h[:], bytes)
	return nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (h Hash) MarshalBinary() ([]byte, error) {
	return h[:], nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (h *Hash) UnmarshalBinary(data []byte) error {
	if len(data) != HashSize {
		return fmt.Errorf("invalid hash length: expected %d bytes, got %d", HashSize, len(data))
	}
	copy(h[:], data)
	return nil
}

// MarshalYAML customizes the YAML marshalling of Hash.
func (h Hash) MarshalYAML() (interface{}, error) {
	return h.Hex(), nil
}

// UnmarshalYAML customizes the YAML unmarshalling of Hash.
func (h *Hash) UnmarshalYAML(value *yaml.Node) error {
	var hexStr string
	if err := value.Decode(&hexStr); err != nil {
		return err
	}
	cleaned := strings.TrimPrefix(hexStr, "0x")
	bytes, err := hex.DecodeString(cleaned)
	if err != nil {
		return err
	}
	if len(bytes) != HashSize {
		return fmt.Errorf("invalid hash length: expected %d bytes, got %d", HashSize, len(bytes))
	}
	copy(h[:], bytes)
	return nil
}
