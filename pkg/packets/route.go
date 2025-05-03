// route.go
package packets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// Define constants for lengths (adjust as per your actual requirements)
const (
	MACLength       = 6  // Length of MAC address in bytes
	SignatureLength = 64 // Example length for Signature (e.g., 512-bit signature)
	IPLength        = 16 // Supports both IPv4 and IPv6
)

// RoutePacket represents a network route packet
type RoutePacket struct {
	NextHopIP  [IPLength]byte // Fixed-size array for IP to simplify serialization
	NextHopMAC []byte         // Optional MAC address
	Signature  []byte         // Optional Signature
}

// SerializeRoutePacket serializes a RoutePacket into a byte slice
func SerializeRoutePacket(pkt *RoutePacket) ([]byte, error) {
	var buf bytes.Buffer

	// Serialize NextHopIP
	if err := binary.Write(&buf, binary.BigEndian, pkt.NextHopIP); err != nil {
		return nil, fmt.Errorf("failed to serialize NextHopIP: %w", err)
	}

	// Serialize NextHopMAC indicator
	if pkt.NextHopMAC != nil {
		buf.WriteByte(1) // Indicator that NextHopMAC is present
		if len(pkt.NextHopMAC) != MACLength {
			return nil, fmt.Errorf("NextHopMAC length is %d, expected %d", len(pkt.NextHopMAC), MACLength)
		}
		if _, err := buf.Write(pkt.NextHopMAC); err != nil {
			return nil, fmt.Errorf("failed to serialize NextHopMAC: %w", err)
		}
	} else {
		buf.WriteByte(0) // Indicator that NextHopMAC is absent
	}

	// Serialize Signature indicator
	if pkt.Signature != nil {
		buf.WriteByte(1) // Indicator that Signature is present
		if len(pkt.Signature) != SignatureLength {
			return nil, fmt.Errorf("Signature length is %d, expected %d", len(pkt.Signature), SignatureLength)
		}
		if _, err := buf.Write(pkt.Signature); err != nil {
			return nil, fmt.Errorf("failed to serialize Signature: %w", err)
		}
	} else {
		buf.WriteByte(0) // Indicator that Signature is absent
	}

	return buf.Bytes(), nil
}

// DeserializeRoutePacket deserializes a byte slice into a RoutePacket
func DeserializeRoutePacket(data []byte) (*RoutePacket, error) {
	pkt := &RoutePacket{}
	reader := bytes.NewReader(data)

	// Deserialize NextHopIP
	if err := binary.Read(reader, binary.BigEndian, &pkt.NextHopIP); err != nil {
		return nil, fmt.Errorf("failed to deserialize NextHopIP: %w", err)
	}

	// Deserialize NextHopMAC indicator
	var hasNextHopMAC byte
	if err := binary.Read(reader, binary.BigEndian, &hasNextHopMAC); err != nil {
		return nil, fmt.Errorf("failed to read NextHopMAC indicator: %w", err)
	}
	if hasNextHopMAC == 1 {
		pkt.NextHopMAC = make([]byte, MACLength)
		if n, err := reader.Read(pkt.NextHopMAC); err != nil {
			return nil, fmt.Errorf("failed to deserialize NextHopMAC: %w", err)
		} else if n != MACLength {
			return nil, errors.New("incomplete NextHopMAC data")
		}
	}

	// Deserialize Signature indicator
	var hasSignature byte
	if err := binary.Read(reader, binary.BigEndian, &hasSignature); err != nil {
		return nil, fmt.Errorf("failed to read Signature indicator: %w", err)
	}
	if hasSignature == 1 {
		pkt.Signature = make([]byte, SignatureLength)
		if n, err := reader.Read(pkt.Signature); err != nil {
			return nil, fmt.Errorf("failed to deserialize Signature: %w", err)
		} else if n != SignatureLength {
			return nil, errors.New("incomplete Signature data")
		}
	}

	// Check for any unexpected extra data
	if reader.Len() != 0 {
		return nil, fmt.Errorf("unexpected extra data: %d bytes remaining", reader.Len())
	}

	return pkt, nil
}

// NewRoutePacket creates a new RoutePacket with mandatory fields
func NewRoutePacket(nextHopIP [IPLength]byte) *RoutePacket {
	return &RoutePacket{
		NextHopIP:  nextHopIP,
		NextHopMAC: nil,
		Signature:  nil,
	}
}
