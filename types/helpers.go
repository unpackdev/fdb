package types

import (
	"crypto/sha256"
	"encoding/binary"
	"time"
)

// Helper function to get the current Unix timestamp.
func unixTimestamp() int64 {
	return time.Now().UnixNano()
}

// Helper function to convert a uint64 to a byte slice.
func uint64ToBytes(num uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, num)
	return b
}

// PublicKeyToAddress derives the Address from the public key.
// This function should implement the logic to derive the address from the public key,
// similar to Ethereum's address derivation (e.g., Keccak-256 hash and take last 20 bytes).
func PublicKeyToAddress(pubKey []byte) (Address, error) {
	var addr Address

	// Using SHA-256 and taking the last 20 bytes
	hash := sha256.Sum256(pubKey)
	copy(addr[:], hash[len(hash)-AddressSize:])

	return addr, nil
}
