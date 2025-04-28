// pkg/packets/network.go

package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"

	"github.com/libp2p/go-libp2p/core/peer"
)

// NetworkPacket represents the structure of packets exchanged in the network.
type NetworkPacket struct {
	Type            PacketType // Type of the packet (Ping, Request, Response)
	SenderID        peer.ID    // ID of the sender
	ReceiverID      peer.ID    // ID of the receiver (optional, can be zero value for broadcasts)
	Payload         []byte     // Payload of the packet
	Signature       []byte     // Signature for the packet
	SignaturePubKey []byte     // Public key corresponding to the signature
}

// SerializeWithoutSignature serializes the NetworkPacket without including the Signature and SignaturePubKey fields.
func (np *NetworkPacket) SerializeWithoutSignature() ([]byte, error) {
	var buffer bytes.Buffer

	// Serialize PacketType
	if err := binary.Write(&buffer, binary.LittleEndian, np.Type); err != nil {
		return nil, fmt.Errorf("failed to serialize packet type: %w", err)
	}

	// Serialize SenderID
	if err := serializePeerID(&buffer, np.SenderID); err != nil {
		return nil, fmt.Errorf("failed to serialize sender ID: %w", err)
	}

	// Serialize ReceiverID
	if err := serializePeerID(&buffer, np.ReceiverID); err != nil {
		return nil, fmt.Errorf("failed to serialize receiver ID: %w", err)
	}

	// Serialize Payload length and Payload
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(np.Payload))); err != nil {
		return nil, fmt.Errorf("failed to serialize payload length: %w", err)
	}
	if _, err := buffer.Write(np.Payload); err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	return buffer.Bytes(), nil
}

// Serialize serializes the NetworkPacket into a byte slice.
// It includes the Signature and SignaturePubKey fields if they are present.
func (np *NetworkPacket) Serialize() ([]byte, error) {
	var buffer bytes.Buffer

	// Serialize PacketType
	if err := binary.Write(&buffer, binary.LittleEndian, np.Type); err != nil {
		return nil, fmt.Errorf("failed to serialize packet type: %w", err)
	}

	// Serialize SenderID
	if err := serializePeerID(&buffer, np.SenderID); err != nil {
		return nil, fmt.Errorf("failed to serialize sender ID: %w", err)
	}

	// Serialize ReceiverID
	if err := serializePeerID(&buffer, np.ReceiverID); err != nil {
		return nil, fmt.Errorf("failed to serialize receiver ID: %w", err)
	}

	// Serialize Payload length and Payload
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(np.Payload))); err != nil {
		return nil, fmt.Errorf("failed to serialize payload length: %w", err)
	}
	if _, err := buffer.Write(np.Payload); err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	// Serialize Signature presence flag and Signature
	if np.Signature != nil && len(np.Signature) > 0 {
		// Indicate that a signature is present
		if err := binary.Write(&buffer, binary.LittleEndian, uint8(1)); err != nil {
			return nil, fmt.Errorf("failed to serialize signature presence flag: %w", err)
		}
		// Serialize Signature length and Signature
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(np.Signature))); err != nil {
			return nil, fmt.Errorf("failed to serialize signature length: %w", err)
		}
		if _, err := buffer.Write(np.Signature); err != nil {
			return nil, fmt.Errorf("failed to serialize signature: %w", err)
		}

		// Serialize SignaturePubKey length and SignaturePubKey
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(len(np.SignaturePubKey))); err != nil {
			return nil, fmt.Errorf("failed to serialize signature public key length: %w", err)
		}
		if _, err := buffer.Write(np.SignaturePubKey); err != nil {
			return nil, fmt.Errorf("failed to serialize signature public key: %w", err)
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
		// Serialize SignaturePubKey length as 0
		if err := binary.Write(&buffer, binary.LittleEndian, uint32(0)); err != nil {
			return nil, fmt.Errorf("failed to serialize empty signature public key length: %w", err)
		}
	}

	return buffer.Bytes(), nil
}

// DeserializeNetworkPacket deserializes a byte slice into a NetworkPacket.
// It handles optional fields like Signature and SignaturePubKey gracefully.
func DeserializeNetworkPacket(data []byte) (*NetworkPacket, error) {
	buffer := bytes.NewReader(data)
	np := &NetworkPacket{}

	// Deserialize PacketType
	if err := binary.Read(buffer, binary.LittleEndian, &np.Type); err != nil {
		return nil, fmt.Errorf("failed to deserialize packet type: %w", err)
	}

	// Deserialize SenderID
	senderID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize sender ID: %w", err)
	}
	np.SenderID = senderID

	// Deserialize ReceiverID
	receiverID, err := deserializePeerID(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize receiver ID: %w", err)
	}
	np.ReceiverID = receiverID

	// Deserialize Payload length and Payload
	var payloadLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &payloadLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize payload length: %w", err)
	}
	np.Payload = make([]byte, payloadLen)
	if _, err := buffer.Read(np.Payload); err != nil {
		return nil, fmt.Errorf("failed to deserialize payload: %w", err)
	}

	// Deserialize Signature presence flag and Signature
	var sigPresence uint8
	if err := binary.Read(buffer, binary.LittleEndian, &sigPresence); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize signature presence flag for type: %s", np.Type)
	}

	var sigLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &sigLen); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize signature length for type: %s", np.Type)
	}

	if sigPresence == 1 && sigLen > 0 {
		signature := make([]byte, sigLen)
		if _, err := buffer.Read(signature); err != nil {
			return nil, fmt.Errorf("failed to deserialize signature: %w", err)
		}
		np.Signature = signature

		// Deserialize SignaturePubKey length and SignaturePubKey
		var pubKeyLen uint32
		if err := binary.Read(buffer, binary.LittleEndian, &pubKeyLen); err != nil {
			return nil, fmt.Errorf("failed to deserialize signature public key length: %w", err)
		}
		if pubKeyLen > 0 {
			pubKey := make([]byte, pubKeyLen)
			if _, err := buffer.Read(pubKey); err != nil {
				return nil, fmt.Errorf("failed to deserialize signature public key: %w", err)
			}
			np.SignaturePubKey = pubKey
		} else {
			np.SignaturePubKey = nil
		}
	} else {
		np.Signature = nil
		np.SignaturePubKey = nil
	}

	return np, nil
}
