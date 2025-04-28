package packets

import (
	"bytes"
	"testing"
)

func TestNetworkPacket_SerializationDeserialization(t *testing.T) {
	tests := []struct {
		name        string
		packet      NetworkPacket
		expectError bool
	}{
		{
			name: "PingPacket with Signature",
			packet: NetworkPacket{
				Type:       PacketTypePing,
				SenderID:   MockPeerID(t, "sender1"),
				ReceiverID: MockPeerID(t, "receiver1"),
				Payload:    []byte("ping payload"),
				Signature:  []byte("signature1"),
			},
			expectError: false,
		},
		{
			name: "RequestPacket without ReceiverID",
			packet: NetworkPacket{
				Type:       PacketTypeRequest,
				SenderID:   MockPeerID(t, "sender2"),
				ReceiverID: MockPeerID(t, ""), // Zero value for broadcast
				Payload:    []byte("request payload"),
				Signature:  nil,
			},
			expectError: false,
		},
		{
			name: "ResponsePacket with Empty Signature",
			packet: NetworkPacket{
				Type:       PacketTypeResponse,
				SenderID:   MockPeerID(t, "sender3"),
				ReceiverID: MockPeerID(t, "receiver3"),
				Payload:    []byte("response payload"),
				Signature:  []byte{},
			},
			expectError: false,
		},
		{
			name: "Unknown PacketType",
			packet: NetworkPacket{
				Type:       PacketTypeUnknown,
				SenderID:   MockPeerID(t, "sender4"),
				ReceiverID: MockPeerID(t, "receiver4"),
				Payload:    []byte("unknown payload"),
				Signature:  []byte("signature4"),
			},
			expectError: false, // Depending on implementation, might not cause an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize the NetworkPacket
			serialized, err := tt.packet.Serialize()
			if (err != nil) != tt.expectError {
				t.Fatalf("Serialize() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError {
				// Deserialize the byte slice back into a NetworkPacket
				deserialized, err := DeserializeNetworkPacket(serialized)
				if (err != nil) != tt.expectError {
					t.Fatalf("DeserializeNetworkPacket() error = %v, expectError %v", err, tt.expectError)
				}

				// Verify that the original and deserialized packets are identical
				if deserialized.Type != tt.packet.Type {
					t.Errorf("Type mismatch: got %v, want %v", deserialized.Type, tt.packet.Type)
				}
				if deserialized.SenderID != tt.packet.SenderID {
					t.Errorf("SenderID mismatch: got %v, want %v", deserialized.SenderID, tt.packet.SenderID)
				}
				if deserialized.ReceiverID != tt.packet.ReceiverID {
					t.Errorf("ReceiverID mismatch: got %v, want %v", deserialized.ReceiverID, tt.packet.ReceiverID)
				}
				if !bytes.Equal(deserialized.Payload, tt.packet.Payload) {
					t.Errorf("Payload mismatch: got %v, want %v", deserialized.Payload, tt.packet.Payload)
				}

				// Verify Signature
				if tt.packet.Signature == nil && deserialized.Signature != nil {
					t.Error("Expected Signature to be nil, but got non-nil")
				} else if tt.packet.Signature != nil && deserialized.Signature != nil {
					if !bytes.Equal(deserialized.Signature, tt.packet.Signature) {
						t.Errorf("Signature mismatch: got %v, want %v", deserialized.Signature, tt.packet.Signature)
					}
				}
			}
		})
	}
}

func TestNetworkPacket_Serialize_EmptyFields(t *testing.T) {
	tests := []struct {
		name        string
		packet      NetworkPacket
		expectError bool
	}{
		{
			name: "All Fields Empty",
			packet: NetworkPacket{
				Type:       PacketTypeUnknown,
				SenderID:   MockPeerID(t, ""), // Previously caused t.Fatal
				ReceiverID: MockPeerID(t, ""), // Previously caused t.Fatal
				Payload:    []byte{},
				Signature:  []byte{},
			},
			expectError: false, // Changed from true since empty PeerIDs are now allowed
		},
		{
			name: "Nil Payload",
			packet: NetworkPacket{
				Type:       PacketTypePing,
				SenderID:   MockPeerID(t, "sender5"),
				ReceiverID: MockPeerID(t, "receiver5"),
				Payload:    nil, // Should be handled gracefully
				Signature:  nil,
			},
			expectError: false,
		},
		{
			name: "Nil ReceiverID for Broadcast",
			packet: NetworkPacket{
				Type:       PacketTypeRequest,
				SenderID:   MockPeerID(t, "sender6"),
				ReceiverID: MockPeerID(t, ""), // Zero value for broadcast
				Payload:    []byte("broadcast payload"),
				Signature:  []byte("signature6"),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Attempt to serialize the NetworkPacket
			_, err := tt.packet.Serialize()
			if (err != nil) != tt.expectError {
				t.Fatalf("Serialize() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestNetworkPacket_RoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		packet      NetworkPacket
		expectError bool
	}{
		{
			name: "RoundTrip PingPacket with Signature",
			packet: NetworkPacket{
				Type:       PacketTypePing,
				SenderID:   MockPeerID(t, "sender7"),
				ReceiverID: MockPeerID(t, "receiver7"),
				Payload:    []byte("ping payload7"),
				Signature:  []byte("signature7"),
			},
			expectError: false,
		},
		{
			name: "RoundTrip RequestPacket without Signature",
			packet: NetworkPacket{
				Type:       PacketTypeRequest,
				SenderID:   MockPeerID(t, "sender8"),
				ReceiverID: MockPeerID(t, "receiver8"),
				Payload:    []byte("request payload8"),
				Signature:  nil,
			},
			expectError: false,
		},
		{
			name: "RoundTrip ResponsePacket with Empty Signature",
			packet: NetworkPacket{
				Type:       PacketTypeResponse,
				SenderID:   MockPeerID(t, "sender9"),
				ReceiverID: MockPeerID(t, "receiver9"),
				Payload:    []byte("response payload9"),
				Signature:  []byte{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize the packet
			serialized, err := tt.packet.Serialize()
			if (err != nil) != tt.expectError {
				t.Fatalf("Serialize() error = %v, expectError %v", err, tt.expectError)
			}

			if !tt.expectError {
				// Deserialize the packet
				deserialized, err := DeserializeNetworkPacket(serialized)
				if (err != nil) != tt.expectError {
					t.Fatalf("DeserializeNetworkPacket() error = %v, expectError %v", err, tt.expectError)
				}

				// Serialize again
				serializedAgain, err := deserialized.Serialize()
				if err != nil {
					t.Fatalf("Serialize() after deserialization error = %v", err)
				}

				// Compare the two serialized byte slices
				if !bytes.Equal(serialized, serializedAgain) {
					t.Errorf("Round-trip serialization mismatch")
				}
			}
		})
	}
}
