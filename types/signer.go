package types

import (
	"fmt"
	"github.com/goccy/go-json"

	"gopkg.in/yaml.v3"
)

// SignerType represents the type of a signer.
type SignerType string

// String returns the string representation of the SignerType.
func (t SignerType) String() string {
	return string(t)
}

// Uint32 returns the uint32 representation of the SignerType.
func (t SignerType) Uint32() uint32 {
	switch t {
	case BlsSignerType:
		return 0
	case Ed25519SignerType:
		return 1
	case SchnorrSignerType:
		return 2
	case Secp256k1SignerType:
		return 3
	case ThresholdBLSSignerType:
		return 4
	default:
		return 0xFFFFFFFF // Represents an unknown SignerType
	}
}

// SignerTypeFromUint32 converts a uint32 to a SignerType.
func SignerTypeFromUint32(u uint32) SignerType {
	switch u {
	case 0:
		return BlsSignerType
	case 1:
		return Ed25519SignerType
	case 2:
		return SchnorrSignerType
	case 3:
		return Secp256k1SignerType
	case 4:
		return ThresholdBLSSignerType
	default:
		return UnknownSignerType
	}
}

const (
	BlsSignerType          SignerType = "bls"
	Ed25519SignerType      SignerType = "ed25519"
	SchnorrSignerType      SignerType = "schnorr"
	Secp256k1SignerType    SignerType = "secp256k1"
	ThresholdBLSSignerType SignerType = "threshold_bls"
	UnknownSignerType      SignerType = "unknown"
)

// MarshalYAML customizes the YAML marshaling of SignerType to uint32.
func (t SignerType) MarshalYAML() (interface{}, error) {
	return t.Uint32(), nil
}

// UnmarshalYAML customizes the YAML unmarshaling of SignerType from uint32.
func (t *SignerType) UnmarshalYAML(value *yaml.Node) error {
	var u uint32
	if err := value.Decode(&u); err != nil {
		return fmt.Errorf("failed to decode SignerType as uint32: %w", err)
	}
	*t = SignerTypeFromUint32(u)
	return nil
}

// MarshalJSON customizes the JSON marshaling of SignerType to uint32.
func (t SignerType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Uint32())
}

// UnmarshalJSON customizes the JSON unmarshaling of SignerType from uint32.
func (t *SignerType) UnmarshalJSON(data []byte) error {
	var u uint32
	if err := json.Unmarshal(data, &u); err != nil {
		return fmt.Errorf("failed to decode SignerType as uint32: %w", err)
	}
	*t = SignerTypeFromUint32(u)
	return nil
}
