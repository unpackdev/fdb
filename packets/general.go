package packets

import (
	"bytes"
	"fmt"
)

// Packet is the interface that all packet types will implement.
type Packet interface {
	Serialize() ([]byte, error)
	Deserialize([]byte) error
	GetType() PacketType
}

// GeneralPacket wraps any Packet with its PacketType.
type GeneralPacket struct {
	Type    PacketType
	Payload []byte
}

// Serialize the GeneralPacket by prepending the PacketType.
func (gp *GeneralPacket) Serialize() ([]byte, error) {
	buf := bytes.Buffer{}
	buf.WriteByte(byte(gp.Type))
	buf.Write(gp.Payload)
	return buf.Bytes(), nil
}

// DeserializeGeneralPacket parses the byte slice into a GeneralPacket.
func DeserializeGeneralPacket(data []byte) (*GeneralPacket, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("data too short to contain PacketType")
	}
	gp := &GeneralPacket{
		Type:    PacketType(data[0]),
		Payload: data[1:],
	}
	return gp, nil
}
