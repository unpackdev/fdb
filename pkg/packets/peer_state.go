package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const PeerStatePacketVersion byte = 1

// PeerStatePacket contains information about the peer's state.
type PeerStatePacket struct {
	Version     byte   // Protocol version
	RequestID   uint64 // Unique ID for request-response matching. 0 for broadcasts.
	BlockHeight uint64 // Current block height of the peer
}

// Serialize serializes the PeerStatePacket into a byte slice.
func (psp *PeerStatePacket) Serialize() ([]byte, error) {
	buffer := new(bytes.Buffer)

	// Serialize Version
	if err := buffer.WriteByte(PeerStatePacketVersion); err != nil {
		return nil, fmt.Errorf("failed to serialize Version: %w", err)
	}

	// Serialize RequestID
	if err := binary.Write(buffer, binary.BigEndian, psp.RequestID); err != nil {
		return nil, fmt.Errorf("failed to serialize RequestID: %w", err)
	}

	// Serialize BlockHeight
	if err := binary.Write(buffer, binary.BigEndian, psp.BlockHeight); err != nil {
		return nil, fmt.Errorf("failed to serialize BlockHeight: %w", err)
	}

	return buffer.Bytes(), nil
}

// DeserializePeerStatePacket deserializes a byte slice into a PeerStatePacket.
func DeserializePeerStatePacket(data []byte) (*PeerStatePacket, error) {
	buffer := bytes.NewReader(data)
	psp := &PeerStatePacket{}

	// Deserialize Version
	version, err := buffer.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize Version: %w", err)
	}
	if version != PeerStatePacketVersion {
		return nil, fmt.Errorf("unsupported PeerStatePacket version: %d", version)
	}
	psp.Version = version

	// Deserialize RequestID
	if err := binary.Read(buffer, binary.BigEndian, &psp.RequestID); err != nil {
		return nil, fmt.Errorf("failed to deserialize RequestID: %w", err)
	}

	// Deserialize BlockHeight
	if err := binary.Read(buffer, binary.BigEndian, &psp.BlockHeight); err != nil {
		return nil, fmt.Errorf("failed to deserialize BlockHeight: %w", err)
	}

	return psp, nil
}
