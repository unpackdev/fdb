// pkg/packets/actor_packet.go

package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/unpackdev/fdb/pkg/types"
)

// ActorPacket represents both request and response types with status codes.
type ActorPacket struct {
	Address             types.Address         // Address of the actor itself
	Status              uint8                 // 0 => proposed, 1 => approved, 2 => rejected
	Message             string                // Optional message providing additional information
	Roles               types.Roles           // Roles or permissions of the actor
	SupportedTransports []types.TransportType // Transport types supported by this actor
	SupportedProtocols  []types.ProtocolType  // Supported protocols by this actor
	SupportedSigners    []types.SignerType    // Supported signers by this actor
	ConsensusPublicKey  []byte                // Consensus Distributed Key Generation (DKG) public key (if actor should be part of consensus)
	PublicKey           []byte                // Serialized public key (only in responses)
}

// Serialize serializes the ActorPacket into a byte slice using binary encoding.
func (ap *ActorPacket) Serialize() ([]byte, error) {
	buffer := new(bytes.Buffer)

	// Serialize Address
	aBytes, abErr := ap.Address.MarshalBinary()
	if abErr != nil {
		return nil, fmt.Errorf("failed to serialize Address: %w", abErr)
	}
	if err := serializeByteSlice(buffer, aBytes); err != nil {
		return nil, fmt.Errorf("failed to serialize Address bytes: %w", err)
	}

	// Serialize Status
	if err := binary.Write(buffer, binary.LittleEndian, ap.Status); err != nil {
		return nil, fmt.Errorf("failed to serialize Status: %w", err)
	}

	// Serialize Message
	if err := serializeString(buffer, ap.Message); err != nil {
		return nil, fmt.Errorf("failed to serialize Message: %w", err)
	}

	// Serialize Roles as strings
	if err := serializeStringSlice(buffer, ap.Roles); err != nil {
		return nil, fmt.Errorf("failed to serialize Roles: %w", err)
	}

	// Serialize SupportedTransports
	if err := serializeUint32Slice(buffer, ap.SupportedTransports, func(t types.TransportType) uint32 {
		return t.Uint32()
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize SupportedTransports: %w", err)
	}

	// Serialize SupportedProtocols
	if err := serializeUint32Slice(buffer, ap.SupportedProtocols, func(p types.ProtocolType) uint32 {
		return p.Uint32()
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize SupportedProtocols: %w", err)
	}

	// Serialize SupportedSigners
	if err := serializeUint32Slice(buffer, ap.SupportedSigners, func(s types.SignerType) uint32 {
		return s.Uint32()
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize SupportedSigners: %w", err)
	}

	// Serialize PublicKey length and PublicKey
	if ap.ConsensusPublicKey == nil {
		ap.ConsensusPublicKey = []byte{} // Ensure PublicKey is not nil
	}
	if err := serializeByteSlice(buffer, ap.ConsensusPublicKey); err != nil {
		return nil, fmt.Errorf("failed to serialize consensus public key: %w", err)
	}

	// Serialize PublicKey length and PublicKey
	if ap.PublicKey == nil {
		ap.PublicKey = []byte{} // Ensure PublicKey is not nil
	}
	if err := serializeByteSlice(buffer, ap.PublicKey); err != nil {
		return nil, fmt.Errorf("failed to serialize PublicKey: %w", err)
	}

	return buffer.Bytes(), nil
}

// DeserializeActorPacket deserializes a byte slice into an ActorPacket using binary encoding.
func DeserializeActorPacket(data []byte) (*ActorPacket, error) {
	buffer := bytes.NewReader(data)
	ap := &ActorPacket{}

	// Deserialize Address
	aBytes, err := deserializeByteSlice(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize Address bytes: %w", err)
	}
	if uaErr := ap.Address.UnmarshalBinary(aBytes); uaErr != nil {
		return nil, fmt.Errorf("failed to unmarshal Address: %w", uaErr)
	}

	// Deserialize Status
	if err := binary.Read(buffer, binary.LittleEndian, &ap.Status); err != nil {
		return nil, fmt.Errorf("failed to deserialize Status: %w", err)
	}

	// Deserialize Message
	message, err := deserializeString(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize Message: %w", err)
	}
	ap.Message = message

	// Deserialize Roles as strings
	roles, err := deserializeStringSlice(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize Roles: %w", err)
	}
	ap.Roles = roles

	// Deserialize SupportedTransports
	transports, err := deserializeUint32Slice(buffer, func(u uint32) types.TransportType {
		return types.TransportTypeFromUint32(u)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize SupportedTransports: %w", err)
	}
	ap.SupportedTransports = transports

	// Deserialize SupportedProtocols
	protocols, err := deserializeUint32Slice(buffer, func(u uint32) types.ProtocolType {
		return types.ProtocolTypeFromUint32(u)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize SupportedProtocols: %w", err)
	}
	ap.SupportedProtocols = protocols

	// Deserialize SupportedSigners
	signers, err := deserializeUint32Slice(buffer, func(u uint32) types.SignerType {
		return types.SignerTypeFromUint32(u)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize SupportedSigners: %w", err)
	}
	ap.SupportedSigners = signers

	// Deserialize ConsensusPublicKey
	consensusPublicKey, err := deserializeByteSlice(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize ConsensusPublicKey: %w", err)
	}
	ap.ConsensusPublicKey = consensusPublicKey

	// Deserialize PublicKey
	publicKey, err := deserializeByteSlice(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize PublicKey: %w", err)
	}
	ap.PublicKey = publicKey

	return ap, nil
}

// Helper functions for serialization/deserialization

// serializeString writes a string to the buffer with its length.
func serializeString(buffer *bytes.Buffer, s string) error {
	strBytes := []byte(s)
	strLen := uint32(len(strBytes))
	if err := binary.Write(buffer, binary.LittleEndian, strLen); err != nil {
		return err
	}
	if _, err := buffer.Write(strBytes); err != nil {
		return err
	}
	return nil
}

// deserializeString reads a string from the buffer based on its length.
func deserializeString(buffer *bytes.Reader) (string, error) {
	var strLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &strLen); err != nil {
		return "", err
	}
	strBytes := make([]byte, strLen)
	if _, err := buffer.Read(strBytes); err != nil {
		return "", err
	}
	return string(strBytes), nil
}

// serializeStringSlice serializes a slice of strings.
func serializeStringSlice(buffer *bytes.Buffer, slice types.Roles) error {
	length := uint32(len(slice))
	if err := binary.Write(buffer, binary.LittleEndian, length); err != nil {
		return err
	}
	for _, role := range slice {
		if err := serializeString(buffer, role.String()); err != nil {
			return err
		}
	}
	return nil
}

// deserializeStringSlice deserializes a slice of strings into types.Roles.
func deserializeStringSlice(buffer *bytes.Reader) (types.Roles, error) {
	var length uint32
	if err := binary.Read(buffer, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	slice := make(types.Roles, length)
	for i := uint32(0); i < length; i++ {
		str, err := deserializeString(buffer)
		if err != nil {
			return nil, err
		}
		slice[i] = types.Role(str)
	}
	return slice, nil
}

// serializeUint32Slice serializes a slice of elements that can be converted to uint32.
func serializeUint32Slice[T any](buffer *bytes.Buffer, slice []T, converter func(T) uint32) error {
	length := uint32(len(slice))
	if err := binary.Write(buffer, binary.LittleEndian, length); err != nil {
		return err
	}
	for _, item := range slice {
		val := converter(item)
		if err := binary.Write(buffer, binary.LittleEndian, val); err != nil {
			return err
		}
	}
	return nil
}

// deserializeUint32Slice deserializes a slice of elements from the buffer using a converter function.
func deserializeUint32Slice[T any](buffer *bytes.Reader, converter func(uint32) T) ([]T, error) {
	var length uint32
	if err := binary.Read(buffer, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	slice := make([]T, length)
	for i := uint32(0); i < length; i++ {
		var val uint32
		if err := binary.Read(buffer, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		slice[i] = converter(val)
	}
	return slice, nil
}

// serializeByteSlice writes a byte slice to the buffer with its length.
func serializeByteSlice(buffer *bytes.Buffer, data []byte) error {
	dataLen := uint32(len(data))
	if err := binary.Write(buffer, binary.LittleEndian, dataLen); err != nil {
		return err
	}
	if _, err := buffer.Write(data); err != nil {
		return err
	}
	return nil
}

// deserializeByteSlice reads a byte slice from the buffer based on its length.
func deserializeByteSlice(buffer *bytes.Reader) ([]byte, error) {
	var dataLen uint32
	if err := binary.Read(buffer, binary.LittleEndian, &dataLen); err != nil {
		return nil, err
	}
	data := make([]byte, dataLen)
	if _, err := buffer.Read(data); err != nil {
		return nil, err
	}
	return data, nil
}
