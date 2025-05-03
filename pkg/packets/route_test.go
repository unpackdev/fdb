package packets

import (
	"bytes"
	"reflect"
	"testing"
)

// Helper function to construct expected byte slices
func buildWant(nextHopIP [IPLength]byte, nextHopMAC []byte, signature []byte) []byte {
	var b []byte

	// Append NextHopIP
	b = append(b, nextHopIP[:]...)

	// Append NextHopMAC indicator and data
	if nextHopMAC != nil {
		b = append(b, 1) // Indicator for presence
		b = append(b, nextHopMAC...)
	} else {
		b = append(b, 0) // Indicator for absence
	}

	// Append Signature indicator and data
	if signature != nil {
		b = append(b, 1) // Indicator for presence
		b = append(b, signature...)
	} else {
		b = append(b, 0) // Indicator for absence
	}

	return b
}

func TestRoutePacket_Serialize_EmptyFields(t *testing.T) {
	tests := []struct {
		name        string
		pkt         *RoutePacket
		want        []byte
		expectError bool
	}{
		{
			name: "All_Fields_Empty",
			pkt: &RoutePacket{
				NextHopIP:  [IPLength]byte{},
				NextHopMAC: nil,
				Signature:  nil,
			},
			want:        buildWant([IPLength]byte{}, nil, nil),
			expectError: false,
		},
		{
			name: "Nil_NextHopMAC",
			pkt: &RoutePacket{
				NextHopIP:  [IPLength]byte{},
				NextHopMAC: nil,
				Signature:  make([]byte, SignatureLength), // Should be exactly SignatureLength bytes
			},
			want:        buildWant([IPLength]byte{}, nil, make([]byte, SignatureLength)),
			expectError: false,
		},
		{
			name: "Nil_Signature",
			pkt: &RoutePacket{
				NextHopIP:  [IPLength]byte{},
				NextHopMAC: []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
				Signature:  nil,
			},
			want:        buildWant([IPLength]byte{}, []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, nil),
			expectError: false,
		},
		{
			name: "Partial_Fields_Empty",
			pkt: &RoutePacket{
				NextHopIP:  [IPLength]byte{},
				NextHopMAC: []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
				Signature:  make([]byte, SignatureLength), // Should be exactly SignatureLength bytes
			},
			want:        buildWant([IPLength]byte{}, []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, make([]byte, SignatureLength)),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serialized, err := SerializeRoutePacket(tt.pkt)
			if (err != nil) != tt.expectError {
				t.Errorf("SerializeRoutePacket() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if !bytes.Equal(serialized, tt.want) {
				t.Errorf("Serialized data mismatch.\nGot:  %v\nWant: %v", serialized, tt.want)
			}

			// Now deserialize and compare
			deserialized, err := DeserializeRoutePacket(serialized)
			if (err != nil) != tt.expectError {
				t.Errorf("DeserializeRoutePacket() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !reflect.DeepEqual(deserialized, tt.pkt) {
				t.Errorf("Deserialized RoutePacket mismatch.\nGot:  %+v\nWant: %+v", deserialized, tt.pkt)
			}
		})
	}
}

func TestRoutePacket_RoundTrip(t *testing.T) {
	// Helper function to create a valid NextHopIP
	createNextHopIP := func() [IPLength]byte {
		var ip [IPLength]byte
		// Example: IPv4 address 192.168.1.1
		ip[12] = 192
		ip[13] = 168
		ip[14] = 1
		ip[15] = 1
		return ip
	}

	tests := []struct {
		name        string
		pkt         *RoutePacket
		expectError bool
	}{
		{
			name: "RoundTrip_Valid_RoutePacket_with_Signature",
			pkt: &RoutePacket{
				NextHopIP:  createNextHopIP(),
				NextHopMAC: []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
				Signature:  make([]byte, SignatureLength), // Populate with valid signature bytes if necessary
			},
			expectError: false,
		},
		{
			name: "RoundTrip_RoutePacket_without_Signature",
			pkt: &RoutePacket{
				NextHopIP:  createNextHopIP(),
				NextHopMAC: []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
				Signature:  nil,
			},
			expectError: false,
		},
		{
			name: "RoundTrip_RoutePacket_with_Empty_Signature",
			pkt: &RoutePacket{
				NextHopIP:  createNextHopIP(),
				NextHopMAC: []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
				Signature:  make([]byte, 0), // Empty signature
			},
			expectError: true, // Adjust based on how you handle empty signatures
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serialized, err := SerializeRoutePacket(tt.pkt)
			if (err != nil) != tt.expectError {
				t.Errorf("SerializeRoutePacket() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if tt.expectError {
				return
			}

			deserialized, err := DeserializeRoutePacket(serialized)
			if (err != nil) != tt.expectError {
				t.Errorf("DeserializeRoutePacket() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !reflect.DeepEqual(deserialized, tt.pkt) {
				t.Errorf("Round-trip serialization mismatch.\nOriginal:      %+v\nDeserialized: %+v", tt.pkt, deserialized)
			}
		})
	}
}
