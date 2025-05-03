// pkg/packets/consensus_packet_test.go

package packets

// import (
// 	"bytes"
// 	"github.com/peerdns/peerd/pkg/types"
// 	"testing"

// 	"github.com/stretchr/testify/require"
// )

// // bytesEqualOrBothNil checks if two byte slices are equal or both nil.
// func bytesEqualOrBothNil(a, b []byte) bool {
// 	if a == nil && b == nil {
// 		return true
// 	}
// 	return bytes.Equal(a, b)
// }

// func TestConsensusPacket_SerializationDeserialization(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		packet      ConsensusPacket
// 		expectError bool
// 	}{
// 		{
// 			name: "ProposalPacket with Signature",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypeProposal,
// 				Signer:         types.ThresholdBLSSignerType,
// 				ProposerID:     MockPeerID(t, "proposer1"),
// 				ValidatorID:    MockPeerID(t, "validator1"),
// 				BlockHash:      []byte("blockhash1"),
// 				BlockData:      []byte("blockdata1"),
// 				Signature:      []byte("aggregatedsignature1"),
// 				PartialSig:     nil,
// 				ShareIndex:     1,
// 				SignatureCount: 0,
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "PartialSignaturePacket without Signature",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypePartialSignature,
// 				Signer:         types.Ed25519SignerType,
// 				ProposerID:     MockPeerID(t, "proposer2"),
// 				ValidatorID:    MockPeerID(t, "validator2"),
// 				BlockHash:      []byte("blockhash2"),
// 				BlockData:      nil, // Not used in PartialSignaturePacket
// 				Signature:      nil,
// 				PartialSig:     []byte("partialsig2"),
// 				ShareIndex:     2,
// 				SignatureCount: 1,
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "AggregatedSignaturePacket with Signature",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypeAggregatedSignature,
// 				Signer:         types.ThresholdBLSSignerType,
// 				ProposerID:     MockPeerID(t, "proposer3"),
// 				ValidatorID:    MockPeerID(t, "validator3"),
// 				BlockHash:      []byte("blockhash3"),
// 				BlockData:      nil, // Should be nil for non-ProposalPacket
// 				Signature:      []byte("aggregatedsignature3"),
// 				PartialSig:     nil,
// 				ShareIndex:     0, // Not used in AggregatedSignaturePacket
// 				SignatureCount: 3,
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "FinalizationPacket without Signature",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypeFinalization,
// 				Signer:         types.UnknownSignerType,
// 				ProposerID:     MockPeerID(t, "proposer4"),
// 				ValidatorID:    MockPeerID(t, "validator4"),
// 				BlockHash:      []byte("blockhash4"),
// 				BlockData:      nil, // Not used in FinalizationPacket
// 				Signature:      nil,
// 				PartialSig:     nil,
// 				ShareIndex:     0,
// 				SignatureCount: 0,
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "Invalid PacketType",
// 			packet: ConsensusPacket{
// 				Type:           PacketType(255), // Invalid PacketType
// 				Signer:         types.UnknownSignerType,
// 				ProposerID:     MockPeerID(t, "proposer5"),
// 				ValidatorID:    MockPeerID(t, "validator5"),
// 				BlockHash:      []byte("blockhash5"),
// 				BlockData:      nil, // Should be nil for non-ProposalPacket
// 				Signature:      []byte("signature5"),
// 				PartialSig:     []byte("partialsig5"),
// 				ShareIndex:     5,
// 				SignatureCount: 2,
// 			},
// 			expectError: false, // Depending on implementation, this might not cause an error
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			// Serialize the ConsensusPacket
// 			serialized, err := tt.packet.Serialize()
// 			if (err != nil) != tt.expectError {
// 				t.Fatalf("Serialize() error = %v, expectError %v", err, tt.expectError)
// 			}

// 			if !tt.expectError {
// 				// Deserialize the byte slice back into a ConsensusPacket
// 				deserialized, err := DeserializeConsensusPacket(serialized)
// 				if (err != nil) != tt.expectError {
// 					t.Fatalf("DeserializeConsensusPacket() error = %v, expectError %v", err, tt.expectError)
// 				}

// 				// Verify that the original and deserialized packets are identical
// 				require.Equal(t, tt.packet.Type, deserialized.Type, "Type mismatch")
// 				require.Equal(t, tt.packet.Signer, deserialized.Signer, "Signer mismatch")
// 				require.Equal(t, tt.packet.ProposerID, deserialized.ProposerID, "ProposerID mismatch")
// 				require.Equal(t, tt.packet.ValidatorID, deserialized.ValidatorID, "ValidatorID mismatch")
// 				require.True(t, bytesEqualOrBothNil(tt.packet.BlockHash, deserialized.BlockHash), "BlockHash mismatch")
// 				require.True(t, bytesEqualOrBothNil(tt.packet.BlockData, deserialized.BlockData), "BlockData mismatch")
// 				require.True(t, bytesEqualOrBothNil(tt.packet.Signature, deserialized.Signature), "Signature mismatch")
// 				require.True(t, bytesEqualOrBothNil(tt.packet.PartialSig, deserialized.PartialSig), "PartialSig mismatch")
// 				require.Equal(t, tt.packet.ShareIndex, deserialized.ShareIndex, "ShareIndex mismatch")
// 				require.Equal(t, tt.packet.SignatureCount, deserialized.SignatureCount, "SignatureCount mismatch")
// 			}
// 		})
// 	}
// }

// func TestConsensusPacket_Serialize_EmptyFields(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		packet      ConsensusPacket
// 		expectError bool
// 	}{
// 		{
// 			name: "All Fields Empty",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypeProposal,
// 				Signer:         types.UnknownSignerType,
// 				ProposerID:     MockPeerID(t, ""), // Empty PeerID
// 				ValidatorID:    MockPeerID(t, ""), // Empty PeerID
// 				BlockHash:      []byte{},
// 				BlockData:      []byte{},
// 				Signature:      []byte{},
// 				PartialSig:     []byte{},
// 				ShareIndex:     0,
// 				SignatureCount: 0,
// 			},
// 			expectError: false, // Changed from true since empty PeerIDs are now allowed
// 		},
// 		{
// 			name: "Nil BlockHash",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypePartialSignature,
// 				Signer:         types.Ed25519SignerType,
// 				ProposerID:     MockPeerID(t, "proposer6"),
// 				ValidatorID:    MockPeerID(t, "validator6"),
// 				BlockHash:      nil, // Should be handled gracefully
// 				BlockData:      nil,
// 				Signature:      nil,
// 				PartialSig:     []byte("partialsig6"),
// 				ShareIndex:     6,
// 				SignatureCount: 1,
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "Nil BlockData in Proposal",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypeProposal,
// 				Signer:         types.ThresholdBLSSignerType,
// 				ProposerID:     MockPeerID(t, "proposer7"),
// 				ValidatorID:    MockPeerID(t, "validator7"),
// 				BlockHash:      []byte("blockhash7"),
// 				BlockData:      nil, // Should serialize BlockData length as 0
// 				Signature:      []byte("aggregatedsignature7"),
// 				PartialSig:     nil,
// 				ShareIndex:     7,
// 				SignatureCount: 0,
// 			},
// 			expectError: false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			// Attempt to serialize the ConsensusPacket
// 			serialized, err := tt.packet.Serialize()
// 			if (err != nil) != tt.expectError {
// 				t.Fatalf("Serialize() error = %v, expectError %v", err, tt.expectError)
// 			}

// 			if !tt.expectError {
// 				// Deserialize the packet
// 				deserialized, err := DeserializeConsensusPacket(serialized)
// 				require.NoError(t, err, "DeserializeConsensusPacket() failed")

// 				// Serialize again
// 				serializedAgain, err := deserialized.Serialize()
// 				require.NoError(t, err, "Serialize() after deserialization failed")

// 				// Compare the two serialized byte slices
// 				require.Equal(t, serialized, serializedAgain, "Round-trip serialization mismatch")

// 				// Additionally, verify that empty slices are treated as nil
// 				require.True(t, bytesEqualOrBothNil(tt.packet.BlockHash, deserialized.BlockHash), "BlockHash mismatch after round-trip")
// 				require.True(t, bytesEqualOrBothNil(tt.packet.BlockData, deserialized.BlockData), "BlockData mismatch after round-trip")
// 				require.True(t, bytesEqualOrBothNil(tt.packet.Signature, deserialized.Signature), "Signature mismatch after round-trip")
// 				require.True(t, bytesEqualOrBothNil(tt.packet.PartialSig, deserialized.PartialSig), "PartialSig mismatch after round-trip")
// 			}
// 		})
// 	}
// }

// func TestConsensusPacket_RoundTrip(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		packet      ConsensusPacket
// 		expectError bool
// 	}{
// 		{
// 			name: "RoundTrip ProposalPacket with Signature",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypeProposal,
// 				Signer:         types.ThresholdBLSSignerType,
// 				ProposerID:     MockPeerID(t, "proposer8"),
// 				ValidatorID:    MockPeerID(t, "validator8"),
// 				BlockHash:      []byte("blockhash8"),
// 				BlockData:      []byte("blockdata8"),
// 				Signature:      []byte("aggregatedsignature8"),
// 				PartialSig:     nil,
// 				ShareIndex:     8,
// 				SignatureCount: 0,
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "RoundTrip PartialSignaturePacket without Signature",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypePartialSignature,
// 				Signer:         types.Ed25519SignerType,
// 				ProposerID:     MockPeerID(t, "proposer9"),
// 				ValidatorID:    MockPeerID(t, "validator9"),
// 				BlockHash:      []byte("blockhash9"),
// 				BlockData:      nil, // Not used in PartialSignaturePacket
// 				Signature:      nil,
// 				PartialSig:     []byte("partialsig9"),
// 				ShareIndex:     9,
// 				SignatureCount: 1,
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "RoundTrip AggregatedSignaturePacket with Signature",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypeAggregatedSignature,
// 				Signer:         types.ThresholdBLSSignerType,
// 				ProposerID:     MockPeerID(t, "proposer10"),
// 				ValidatorID:    MockPeerID(t, "validator10"),
// 				BlockHash:      []byte("blockhash10"),
// 				BlockData:      nil, // Should be nil for non-ProposalPacket
// 				Signature:      []byte("aggregatedsignature10"),
// 				PartialSig:     nil,
// 				ShareIndex:     0, // Not used in AggregatedSignaturePacket
// 				SignatureCount: 3,
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "RoundTrip FinalizationPacket without Signature",
// 			packet: ConsensusPacket{
// 				Type:           PacketTypeFinalization,
// 				Signer:         types.UnknownSignerType,
// 				ProposerID:     MockPeerID(t, "proposer11"),
// 				ValidatorID:    MockPeerID(t, "validator11"),
// 				BlockHash:      []byte("blockhash11"),
// 				BlockData:      nil, // Not used in FinalizationPacket
// 				Signature:      nil,
// 				PartialSig:     nil,
// 				ShareIndex:     0,
// 				SignatureCount: 0,
// 			},
// 			expectError: false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			// Serialize the packet
// 			serialized, err := tt.packet.Serialize()
// 			require.NoError(t, err, "Serialize() failed")

// 			// Deserialize the packet
// 			deserialized, err := DeserializeConsensusPacket(serialized)
// 			require.NoError(t, err, "DeserializeConsensusPacket() failed")

// 			// Serialize again
// 			serializedAgain, err := deserialized.Serialize()
// 			require.NoError(t, err, "Serialize() after deserialization failed")

// 			// Compare the two serialized byte slices
// 			require.Equal(t, serialized, serializedAgain, "Round-trip serialization mismatch")

// 			// Additionally, verify that empty slices are treated as nil
// 			require.True(t, bytesEqualOrBothNil(tt.packet.BlockHash, deserialized.BlockHash), "BlockHash mismatch after round-trip")
// 			require.True(t, bytesEqualOrBothNil(tt.packet.BlockData, deserialized.BlockData), "BlockData mismatch after round-trip")
// 			require.True(t, bytesEqualOrBothNil(tt.packet.Signature, deserialized.Signature), "Signature mismatch after round-trip")
// 			require.True(t, bytesEqualOrBothNil(tt.packet.PartialSig, deserialized.PartialSig), "PartialSig mismatch after round-trip")
// 			require.Equal(t, tt.packet.ShareIndex, deserialized.ShareIndex, "ShareIndex mismatch after round-trip")
// 			require.Equal(t, tt.packet.SignatureCount, deserialized.SignatureCount, "SignatureCount mismatch after round-trip")
// 		})
// 	}
// }
