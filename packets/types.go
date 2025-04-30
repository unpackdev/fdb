package packets

import "fmt"

// PacketType represents the type of a packet, encompassing both consensus and network messages.
type PacketType uint8

func (t PacketType) String() string {
	switch t {
	case PacketTypeUnknown:
		return "unknown"
	case ActorPacketType:
		return "topology_actor"
	case PacketTypeProposal:
		return "proposal"
	case PacketTypeApproval:
		return "approval"
	case PacketTypeFinalization:
		return "finalization"
	case PacketTypePartialSignature:
		return "partial_signature"
	case PacketTypeAggregatedSignature:
		return "aggregated_signature"
	case PacketTypePing:
		return "ping"
	case PacketTypeRequest:
		return "request"
	case PacketTypeResponse:
		return "response"
	case PacketTypePeerState:
		return "peer_state"
	case PacketTypeSnapshotMetadataRequest:
		return "snapshot_metadata_request"
	case PacketTypeSnapshotMetadataResponse:
		return "snapshot_metadata_response"
	case PacketTypeSnapshotRequest:
		return "snapshot_request"
	case PacketTypeSnapshotChunk:
		return "snapshot_chunk"
	case PacketDealBundle:
		return "dkg_deal_bundle"
	case PacketResponseBundle:
		return "dkg_response_bundle"
	case PacketJustificationBundle:
		return "dkg_justification_bundle"
	case PacketBlockProduction:
		return "block_production"
	case PacketTypeValidatorJoin:
		return "validator_join"
	case PacketTypeValidatorLeave:
		return "validator_leave"
	case PacketTypeCommitment:
		return "commitment"
	case PacketTypeShare:
		return "share"
	case PacketTypeBlockRequest:
		return "block_request"
	case PacketTypeBlockResponse:
		return "block_response"
	case PacketTypePeerStateRequest:
		return "peer_state_request"
	case PacketTypePeerStateResponse:
		return "peer_state_response"
	default:
		return "unknown"
	}
}

func (t PacketType) Bytes() []byte {
	return []byte{byte(t)}
}

// FromBytes converts a byte slice to PacketType.
func (t *PacketType) FromBytes(data []byte) error {
	if len(data) < 1 {
		return fmt.Errorf("insufficient data to parse PacketType")
	}
	*t = PacketType(data[0])
	return nil
}

// GetPacketType reads the first byte from the data to determine the PacketType.
func GetPacketType(data []byte) (PacketType, error) {
	if len(data) == 0 {
		return PacketTypeUnknown, fmt.Errorf("data is empty")
	}
	return PacketType(data[0]), nil
}

const (
	PacketTypeUnknown PacketType = 0 // Represents an unknown message type

	// Network exchange packets
	ActorPacketType PacketType = 1

	// Consensus Packet Types
	PacketTypeProposal            PacketType = 4 // Indicates a new block proposal
	PacketTypeApproval            PacketType = 5 // Indicates approval of a proposal
	PacketTypeFinalization        PacketType = 6 // Indicates block finalization
	PacketTypePartialSignature    PacketType = 7
	PacketTypeAggregatedSignature PacketType = 8

	// Network Packet Types
	PacketTypePing     PacketType = 9  // Represents a simple ping message
	PacketTypeRequest  PacketType = 10 // Represents a request message
	PacketTypeResponse PacketType = 11 // Represents a response message

	// State synchronization and snapshots
	PacketTypePeerState                PacketType = 12
	PacketTypeSnapshotMetadataRequest  PacketType = 13
	PacketTypeSnapshotMetadataResponse PacketType = 14
	PacketTypeSnapshotRequest          PacketType = 15
	PacketTypeSnapshotChunk            PacketType = 16

	// Distributed Key Generation packets
	PacketDealBundle          PacketType = 17
	PacketResponseBundle      PacketType = 18
	PacketJustificationBundle PacketType = 19

	// Block production
	PacketBlockProduction PacketType = 20

	// Validator Packet Types
	PacketTypeValidatorJoin  PacketType = 21
	PacketTypeValidatorLeave PacketType = 22
	PacketTypeCommitment     PacketType = 23
	PacketTypeShare          PacketType = 24

	// Block synchronization packets
	PacketTypeBlockRequest      PacketType = 25
	PacketTypeBlockResponse     PacketType = 26
	PacketTypePeerStateRequest  PacketType = 27
	PacketTypePeerStateResponse PacketType = 28

	// Record distribution packets
	RecordBatchType             PacketType = 30 // Batch of database records for P2P distribution
)
