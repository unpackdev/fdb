package types

import (
	"encoding/hex"
	"encoding/json"
	"gopkg.in/yaml.v3"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressMarshalUnmarshal(t *testing.T) {
	addresses := []Address{
		MustFromHex("0x1234567890abcdef1234567890abcdef12345678"),
		MustFromHex("0xabcdef1234567890abcdef1234567890abcdef12"),
		MustFromHex("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"),
	}

	for _, addr := range addresses {
		t.Run(addr.Hex(), func(t *testing.T) {
			// Marshal to YAML
			data, err := yaml.Marshal(addr)
			if err != nil {
				t.Fatalf("yaml.Marshal() error = %v", err)
			}

			// Unmarshal back
			var unmarshalled Address
			err = yaml.Unmarshal(data, &unmarshalled)
			if err != nil {
				t.Fatalf("yaml.Unmarshal() error = %v", err)
			}

			// Compare original and unmarshalled addresses
			if !addr.Equals(unmarshalled) {
				t.Errorf("Addresses do not match. Got %v, want %v", unmarshalled.Hex(), addr.Hex())
			}
		})
	}
}

func TestAddressCreation(t *testing.T) {
	validBytes := make([]byte, AddressSize)
	for i := 0; i < AddressSize; i++ {
		validBytes[i] = byte(i)
	}
	invalidBytes := make([]byte, AddressSize-1)

	tests := []struct {
		name      string
		function  func() (interface{}, error)
		expectErr bool
		check     func(t *testing.T, result interface{})
	}{
		{
			name: "NewAddress with valid bytes",
			function: func() (interface{}, error) {
				return NewAddress(validBytes)
			},
			expectErr: false,
			check: func(t *testing.T, result interface{}) {
				addr := result.(Address)
				assert.Equal(t, validBytes, addr.Bytes())
			},
		},
		{
			name: "NewAddress with invalid bytes",
			function: func() (interface{}, error) {
				return NewAddress(invalidBytes)
			},
			expectErr: true,
		},
		{
			name: "FromBytes with valid bytes",
			function: func() (interface{}, error) {
				return FromBytes(validBytes), nil
			},
			expectErr: false,
			check: func(t *testing.T, result interface{}) {
				addr := result.(Address)
				assert.Equal(t, validBytes, addr.Bytes())
			},
		},
		{
			name: "FromHex with valid hex",
			function: func() (interface{}, error) {
				hexStr := "0x" + hex.EncodeToString(validBytes)
				return FromHex(hexStr)
			},
			expectErr: false,
			check: func(t *testing.T, result interface{}) {
				addr := result.(Address)
				assert.Equal(t, validBytes, addr.Bytes())
			},
		},
		{
			name: "FromHex with invalid length",
			function: func() (interface{}, error) {
				hexStr := "0x1234"
				return FromHex(hexStr)
			},
			expectErr: true,
		},
		{
			name: "FromHex with invalid hex",
			function: func() (interface{}, error) {
				hexStr := "0xZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ"
				return FromHex(hexStr)
			},
			expectErr: true,
		},
		{
			name: "MustFromHex with valid hex",
			function: func() (interface{}, error) {
				hexStr := "0x" + hex.EncodeToString(validBytes)
				return MustFromHex(hexStr), nil
			},
			expectErr: false,
			check: func(t *testing.T, result interface{}) {
				addr := result.(Address)
				assert.Equal(t, validBytes, addr.Bytes())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.function()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, result)
				}
			}
		})
	}
}

func TestAddressJSONMarshalling(t *testing.T) {
	bytes := []byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x11, 0x22,
		0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc}
	addr, err := NewAddress(bytes)
	require.NoError(t, err)

	tests := []struct {
		name      string
		function  func() (interface{}, error)
		expectErr bool
		check     func(t *testing.T, result interface{})
	}{
		{
			name: "Marshal to JSON",
			function: func() (interface{}, error) {
				return json.Marshal(addr)
			},
			expectErr: false,
			check: func(t *testing.T, result interface{}) {
				data := result.([]byte)
				expected := `"0x123456789abcdef0112233445566778899aabbcc"`
				assert.JSONEq(t, expected, string(data))
			},
		},
		{
			name: "Unmarshal from JSON",
			function: func() (interface{}, error) {
				jsonStr := `"0x123456789abcdef0112233445566778899aabbcc"`
				var newAddr Address
				err := json.Unmarshal([]byte(jsonStr), &newAddr)
				return newAddr, err
			},
			expectErr: false,
			check: func(t *testing.T, result interface{}) {
				newAddr := result.(Address)
				assert.True(t, addr.Equals(newAddr))
			},
		},
		{
			name: "Unmarshal invalid JSON",
			function: func() (interface{}, error) {
				jsonStr := `"0x1234"`
				var newAddr Address
				err := json.Unmarshal([]byte(jsonStr), &newAddr)
				return nil, err
			},
			expectErr: true,
		},
		{
			name: "Marshal and Unmarshal consistency",
			function: func() (interface{}, error) {
				data, err := json.Marshal(addr)
				if err != nil {
					return nil, err
				}

				var unmarshaledAddr Address
				err = json.Unmarshal(data, &unmarshaledAddr)
				return unmarshaledAddr, err
			},
			expectErr: false,
			check: func(t *testing.T, result interface{}) {
				unmarshaledAddr := result.(Address)
				assert.True(t, addr.Equals(unmarshaledAddr))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.function()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, result)
				}
			}
		})
	}
}
