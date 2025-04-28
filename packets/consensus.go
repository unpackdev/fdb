// pkg/packets/consensus_packet.go

package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/peerdns/peerd/pkg/types"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ConsensusPacket represents a packet exchanged during the consensus protocol.
type ConsensusPacket struct {
	Type       PacketType       // Type of the packet (Proposal, PartialSignature, AggregatedSignature, Finalization)
	Signer     types.SignerType // Signer type used
	ProposerID peer.ID          // ID of the validator proposing the block (for ProposalPacket)
	//ValidatorID      peer.ID          // ID of the validator approving the proposal (for PartialSignaturePacket)
	BlockHash  types.Hash // Hash of the block involved in the packet
	BlockData  []byte     // Raw block data (optional, used in ProposalPacket)
	Signature  []byte     // Aggregated signature (used in AggregatedSignaturePacket)
	PartialSig []byte     // Partial signature (used in PartialSignaturePacket)
	//ParticipantIndex uint32     // Participant index (used in partial signatures)
	SignatureSize int // Size of partial signatures received (optional)
}

// Serialize serializes a ConsensusPacket into a byte slice.
func (cp *ConsensusPacket) Serialize() ([]byte, error) {
	var buffer bytes.Buffer

	// Serialize the PacketType
	if err := binary.Write(&buffer, binary.LittleEndian, cp.Type); err != nil {
		return nil, fmt.Errorf("failed to serialize packet type: %w", err)
	}

	// Serialize the Signer as uint32
	signerUint := cp.Signer.Uint32()
	if err := binary.Write(&buffer, binary.LittleEndian, signerUint); err != nil {
		return nil, fmt.Errorf("failed to serialize signer: %w", err)
	}

	// Serialize ProposerID and ValidatorID
	if err := serializePeerID(&buffer, cp.ProposerID); err != nil {
		return nil, fmt.Errorf("failed to serialize proposer ID: %w", err)
	}
	/*	if err := serializePeerID(&buffer, cp.ValidatorID); err != nil {
		return nil, fmt.Errorf("failed to serialize validator ID: %w", err)
	}*/

	// Serialize BlockHash length and BlockHash
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cp.BlockHash.Bytes()))); err != nil {
		return nil, fmt.Errorf("failed to serialize block hash length: %w", err)
	}
	if len(cp.BlockHash.Bytes()) > 0 {
		if _, err := buffer.Write(cp.BlockHash.Bytes()); err != nil {
			return nil, fmt.Errorf("failed to serialize block hash: %w", err)
		}
	}

	// Serialize BlockData length and BlockData (if applicable)
	if cp.Type == PacketTypeProposal {
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cp.BlockData))); err != nil {
			return nil, fmt.Errorf("failed to serialize block data length: %w", err)
		}
		if len(cp.BlockData) > 0 {
			if _, err := buffer.Write(cp.BlockData); err != nil {
				return nil, fmt.Errorf("failed to serialize block data: %w", err)
			}
		}
	} else {
		// For non-ProposalPacket types, serialize block data length as 0
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize empty block data length: %w", err)
		}
	}

	// Serialize Signature presence flag and Signature
	if cp.Signature != nil && len(cp.Signature) > 0 {
		// Indicate that a signature is present
		if err := binary.Write(&buffer, binary.LittleEndian, uint8(1)); err != nil {
			return nil, fmt.Errorf("failed to serialize signature presence flag: %w", err)
		}
		// Serialize Signature length and Signature
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cp.Signature))); err != nil {
			return nil, fmt.Errorf("failed to serialize signature length: %w", err)
		}
		if _, err := buffer.Write(cp.Signature); err != nil {
			return nil, fmt.Errorf("failed to serialize signature: %w", err)
		}
	} else {
		// Indicate that no signature is present
		if err := binary.Write(&buffer, binary.LittleEndian, uint8(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize signature absence flag: %w", err)
		}
		// Serialize Signature length as 0
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize empty signature length: %w", err)
		}
	}

	// Serialize PartialSig presence flag and PartialSig
	if cp.PartialSig != nil && len(cp.PartialSig) > 0 {
		// Indicate that a partial signature is present
		if err := binary.Write(&buffer, binary.LittleEndian, uint8(1)); err != nil {
			return nil, fmt.Errorf("failed to serialize partial signature presence flag: %w", err)
		}
		// Serialize PartialSig length and PartialSig
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(cp.PartialSig))); err != nil {
			return nil, fmt.Errorf("failed to serialize partial signature length: %w", err)
		}
		if _, err := buffer.Write(cp.PartialSig); err != nil {
			return nil, fmt.Errorf("failed to serialize partial signature: %w", err)
		}
	} else {
		// Indicate that no partial signature is present
		if err := binary.Write(&buffer, binary.LittleEndian, uint8(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize partial signature absence flag: %w", err)
		}
		// Serialize PartialSig length as 0
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize empty partial signature length: %w", err)
		}
	}

	/*	// Serialize ParticipantIndex
		if err := binary.Write(&buffer, binary.LittleEndian, int32(cp.ParticipantIndex)); err != nil {
			return nil, fmt.Errorf("failed to serialize share index: %w", err)
		}*/

	// Serialize SignatureSize
	if err := binary.Write(&buffer, binary.LittleEndian, int32(cp.SignatureSize)); err != nil {
		return nil, fmt.Errorf("failed to serialize signature count: %w", err)
	}

	return buffer.Bytes(), nil
}

// DeserializeConsensusPacket deserializes a byte slice into a ConsensusPacket.
func DeserializeConsensusPacket(data []byte) (*ConsensusPacket, error) {
	buffer := bytes.NewBuffer(data)
	cp := &ConsensusPacket{}

	// Deserialize PacketType
	if err := binary.Read(buffer, binary.LittleEndian, &cp.Type); err != nil {
		return nil, fmt.Errorf("failed to deserialize packet type: %w", err)
	}

	// Deserialize Signer from uint32
	var signerUint uint32
	if err := binary.Read(buffer, binary.LittleEndian, &signerUint); err != nil {
		return nil, fmt.Errorf("failed to deserialize signer: %w", err)
	}
	cp.Signer = types.SignerTypeFromUint32(signerUint)

	// Deserialize ProposerID and ValidatorID
	proposerID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize proposer ID: %w", err)
	}
	cp.ProposerID = proposerID

	/*	validatorID, err := deserializePeerID(buffer)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize validator ID: %w", err)
		}
		cp.ValidatorID = validatorID
	*/
	// Deserialize BlockHash length and BlockHash
	var blockHashLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &blockHashLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize block hash length: %w", err)
	}
	if blockHashLen > 0 {
		blockHashB := make([]byte, blockHashLen)
		if _, bErr := buffer.Read(blockHashB); bErr != nil {
			return nil, fmt.Errorf("failed to deserialize block hash: %w", bErr)
		}
		blockHash, bhErr := types.HashFromBytes(blockHashB)
		if bhErr != nil {
			return nil, fmt.Errorf("failed to deserialize block hash: %w", err)
		}
		cp.BlockHash = blockHash
	} else {
		cp.BlockHash = types.ZeroHash
	}

	// Deserialize BlockData length and BlockData
	var blockDataLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &blockDataLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize block data length: %w", err)
	}
	if blockDataLen > 0 {
		cp.BlockData = make([]byte, blockDataLen)
		if _, err := buffer.Read(cp.BlockData); err != nil {
			return nil, fmt.Errorf("failed to deserialize block data: %w", err)
		}
	} else {
		cp.BlockData = nil
	}

	// Deserialize Signature presence flag and Signature
	var sigPresence uint8
	if err := binary.Read(buffer, binary.LittleEndian, &sigPresence); err != nil {
		return nil, fmt.Errorf("failed to deserialize signature presence flag: %w", err)
	}

	var sigLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &sigLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize signature length: %w", err)
	}

	if sigPresence == 1 && sigLen > 0 {
		signature := make([]byte, sigLen)
		if _, err := buffer.Read(signature); err != nil {
			return nil, fmt.Errorf("failed to deserialize signature: %w", err)
		}
		cp.Signature = signature
	} else {
		cp.Signature = nil
	}

	// Deserialize PartialSig presence flag and PartialSig
	var partialSigPresence uint8
	if err := binary.Read(buffer, binary.LittleEndian, &partialSigPresence); err != nil {
		return nil, fmt.Errorf("failed to deserialize partial signature presence flag: %w", err)
	}

	var partialSigLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &partialSigLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize partial signature length: %w", err)
	}

	if partialSigPresence == 1 && partialSigLen > 0 {
		partialSig := make([]byte, partialSigLen)
		if _, err := buffer.Read(partialSig); err != nil {
			return nil, fmt.Errorf("failed to deserialize partial signature: %w", err)
		}
		cp.PartialSig = partialSig
	} else {
		cp.PartialSig = nil
	}

	/*	// Deserialize ParticipantIndex
		var participantIndex uint32
		if err := binary.Read(buffer, binary.LittleEndian, &participantIndex); err != nil {
			return nil, fmt.Errorf("failed to deserialize share index: %w", err)
		}
		cp.ParticipantIndex = participantIndex
	*/
	// Deserialize SignatureSize
	var sigSize int32
	if err := binary.Read(buffer, binary.LittleEndian, &sigSize); err != nil {
		return nil, fmt.Errorf("failed to deserialize signature count: %w", err)
	}
	cp.SignatureSize = int(sigSize)

	return cp, nil
}
