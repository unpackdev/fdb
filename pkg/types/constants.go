package types

const (
	// HashSize defines the size of a SHA-256 hash in bytes.
	HashSize = 32

	// AddressSize defines the size of an address or public key in bytes.
	AddressSize = 20

	// MaxPayloadSize defines the maximum allowed payload size in bytes.
	MaxPayloadSize = 1 << 20 // 1 MB maximum payload size to prevent excessive memory usage
)
