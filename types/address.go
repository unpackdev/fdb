package types

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
	"gopkg.in/yaml.v3"
	"strings"
)

// ErrInvalidAddressLength is returned when the address does not have the correct length.
var ErrInvalidAddressLength = errors.New("invalid address length")

// Address represents a fixed-size 20-byte Ethereum-like address.
type Address [AddressSize]byte

func IsZeroAddress(h Address) bool {
	zeroBytes := make([]byte, AddressSize)
	return bytes.Equal(h[:], zeroBytes)
}

// NewAddress creates a new Address from a byte slice.
// Returns an error if the slice is not exactly 20 bytes.
func NewAddress(bytes []byte) (Address, error) {
	var addr Address
	if len(bytes) != AddressSize {
		return addr, ErrInvalidAddressLength
	}
	copy(addr[:], bytes)
	return addr, nil
}

// AddressFromBytes creates an Address from a byte slice.
// Panics if the slice is not exactly 20 bytes.
func AddressFromBytes(bytes []byte) Address {
	if len(bytes) != AddressSize {
		panic(fmt.Sprintf("invalid address length: expected %d bytes, got %d", AddressSize, len(bytes)))
	}
	var addr Address
	copy(addr[:], bytes)
	return addr
}

// FromCommonAddress converts common.Address to types.Address.
func FromCommonAddress(cAddr common.Address) Address {
	return Address(cAddr)
}

// AddressFromHex creates an Address from a hex string.
// The hex string may have a "0x" prefix.
// Returns an error if the string is not valid hex or does not represent exactly 20 bytes.
func AddressFromHex(s string) (Address, error) {
	var addr Address
	cleaned := strings.TrimPrefix(s, "0x")
	if len(cleaned) != AddressSize*2 {
		return addr, ErrInvalidAddressLength
	}
	aBytes, err := hex.DecodeString(cleaned)
	if err != nil {
		return addr, err
	}
	copy(addr[:], aBytes)
	return addr, nil
}

// MustAddressFromHex creates an Address from a hex string.
// Panics if the string is not a valid hex or does not represent exactly 20 bytes.
func MustAddressFromHex(s string) Address {
	addr, err := AddressFromHex(s)
	if err != nil {
		panic(err)
	}
	return addr
}

// Bytes returns the byte representation of the Address.
func (a Address) Bytes() []byte {
	aBytes := make([]byte, AddressSize)
	copy(aBytes, a[:])
	return aBytes
}

// Hex returns the EIP-55 checksummed hexadecimal string representation of the address.
func (a Address) Hex() string {
	// Step 1: Convert the address bytes to lowercase hex string without '0x' prefix.
	addrHex := hex.EncodeToString(a[:])

	// Step 2: Compute the Keccak-256 hash of the lowercase hex address.
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(addrHex))
	hash := hasher.Sum(nil)

	// Step 3: Apply EIP-55 checksum.
	checksumAddress := "0x"
	for i := 0; i < len(addrHex); i++ {
		c := addrHex[i]
		// For letters (a-f), check the corresponding hash nibble.
		if c >= 'a' && c <= 'f' {
			// Each character corresponds to 4 bits in the hash.
			// For each character, determine if the corresponding hash nibble is >= 8.
			hashByte := hash[i/2]
			var hashNibble byte
			if i%2 == 0 {
				// Even index, high nibble
				hashNibble = hashByte >> 4
			} else {
				// Odd index, low nibble
				hashNibble = hashByte & 0x0F
			}
			if hashNibble >= 8 {
				// Uppercase the character
				checksumAddress += strings.ToUpper(string(c))
			} else {
				// Keep the character lowercase
				checksumAddress += string(c)
			}
		} else {
			// For numbers, just add the character
			checksumAddress += string(c)
		}
	}

	return checksumAddress
}

// String returns the hexadecimal string representation of the Address with "0x" prefix.
// Implements the fmt.Stringer interface.
func (a Address) String() string {
	return a.Hex()
}

// Equals compares two Addresses for equality.
func (a Address) Equals(other Address) bool {
	for i := 0; i < AddressSize; i++ {
		if a[i] != other[i] {
			return false
		}
	}
	return true
}

// ToCommonAddress converts types.Address to common.Address.
// This is useful for interaction with go-ethereum client.
// Otherwise types.Address and common.Address are identical in value.
func (a Address) ToCommonAddress() common.Address {
	return common.Address(a)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (a Address) MarshalText() ([]byte, error) {
	return []byte(a.Hex()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (a *Address) UnmarshalText(text []byte) error {
	parsed, err := AddressFromHex(string(text))
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (a Address) MarshalJSON() ([]byte, error) {
	return []byte(`"` + a.Hex() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (a *Address) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), `"`)
	parsed, err := AddressFromHex(str)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (a Address) MarshalBinary() ([]byte, error) {
	return a.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (a *Address) UnmarshalBinary(data []byte) error {
	if len(data) != AddressSize {
		return ErrInvalidAddressLength
	}
	copy(a[:], data)
	return nil
}

// MarshalYAML customizes the YAML marshalling of Address.
func (a Address) MarshalYAML() (interface{}, error) {
	return a.Hex(), nil
}

// UnmarshalYAML customizes the YAML unmarshalling of Address.
func (a *Address) UnmarshalYAML(value *yaml.Node) error {
	var hexStr string
	if err := value.Decode(&hexStr); err != nil {
		return err
	}
	parsed, err := AddressFromHex(hexStr)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// IsZero checks if the address is the zero address.
func (a Address) IsZero() bool {
	for _, b := range a {
		if b != 0 {
			return false
		}
	}
	return true
}

// ShortHex returns a shortened version of the address for display purposes.
func (a Address) ShortHex() string {
	if len(a.Hex()) < 10 {
		return a.Hex()
	}
	return a.Hex()[:6] + "..." + a.Hex()[len(a.Hex())-4:]
}

// IsHexAddress verifies whether a string can represent a valid hex-encoded
// address or not.
func IsHexAddress(s string) bool {
	if has0xPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*AddressSize && isHex(s)
}

// has0xPrefix validates str begins with '0x' or '0X'.
func has0xPrefix(str string) bool {
	return len(str) >= 2 && str[0] == '0' && (str[1] == 'x' || str[1] == 'X')
}

// isHexCharacter returns bool of c being a valid hexadecimal.
func isHexCharacter(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// isHex validates whether each byte is valid hexadecimal string.
func isHex(str string) bool {
	if len(str)%2 != 0 {
		return false
	}
	for _, c := range []byte(str) {
		if !isHexCharacter(c) {
			return false
		}
	}
	return true
}
