package packets

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"net"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	mh "github.com/multiformats/go-multihash"
)

// MockPeerID creates a mock peer.ID from a string.
// It now allows empty strings to represent zero peer.ID without causing the test to fail.
func MockPeerID(t *testing.T, idStr string) peer.ID {
	if idStr == "" {
		return ""
	}
	// Encode the string into a multihash using SHA2-256
	mhBytes, err := mh.Encode([]byte(idStr), mh.SHA2_256)
	require.NoError(t, err, "Failed to encode multihash for MockPeerID")

	// Create a peer.ID from the multihash bytes
	pid, err := peer.IDFromBytes(mhBytes)
	require.NoError(t, err, "Failed to create peer.ID from bytes in MockPeerID")

	return pid
}

// Equal checks if two byte slices are equal.
func Equal(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// MockMACAddress creates a mock MAC address.
func MockMACAddress() net.HardwareAddr {
	return net.HardwareAddr{0x00, 0x1A, 0x2B, 0x3C, 0x4D, 0x5E}
}

// MockIPAddress creates a mock IP address (IPv6 for compatibility).
func MockIPAddress(ipStr string) net.IP {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return net.IPv6zero
	}
	return ip.To16()
}
