// signatures/secp256k1_test.go

package signatures

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecp256k1Signer_SignAndVerify(t *testing.T) {
	signer, err := NewSecp256k1Signer()
	require.NoError(t, err, "Failed to initialize Secp256k1Signer")

	tests := []struct {
		name            string
		data            []byte
		alterData       bool
		expectSignErr   bool
		expectValid     bool
		expectVerifyErr bool
	}{
		{
			name:            "Valid signature",
			data:            []byte("This is a sample message for signing."),
			alterData:       false,
			expectSignErr:   false,
			expectValid:     true,
			expectVerifyErr: false,
		},
		{
			name:            "Invalid signature with altered data",
			data:            []byte("This is a sample message for signing."),
			alterData:       true,
			expectSignErr:   false,
			expectValid:     false,
			expectVerifyErr: false,
		},
		{
			name:            "Empty data",
			data:            []byte{},
			alterData:       false,
			expectSignErr:   true,
			expectValid:     false,
			expectVerifyErr: true,
		},
		{
			name:            "Nil data",
			data:            nil,
			alterData:       false,
			expectSignErr:   true,
			expectValid:     false,
			expectVerifyErr: true,
		},
		{
			name:            "Invalid signature length",
			data:            []byte("Valid data"),
			alterData:       false,
			expectSignErr:   false,
			expectValid:     false,
			expectVerifyErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataToSign := tt.data
			dataToVerify := tt.data
			signature := []byte{}

			if tt.alterData && len(tt.data) > 0 {
				// Create a copy of the data and alter it for verification
				dataToVerify = append([]byte{}, tt.data...)
				dataToVerify[0] ^= 0xFF // Alter the data
			}

			if tt.name != "Invalid signature length" {
				signature, err = signer.Sign(dataToSign)
				if tt.expectSignErr {
					assert.Error(t, err, "Expected signing to fail")
					return
				} else {
					assert.NoError(t, err, "Expected signing to succeed")
					assert.Len(t, signature, 65, "Signature should be 65 bytes long")
				}
			} else {
				// Create an invalid signature with incorrect length
				signature = []byte("invalid_signature")
			}

			valid, err := signer.Verify(dataToVerify, signature)
			if tt.expectVerifyErr {
				assert.Error(t, err, "Expected verification to error")
				assert.False(t, valid, "Expected verification to fail")
			} else {
				assert.NoError(t, err, "Verification should not error")
				assert.Equal(t, tt.expectValid, valid, "Signature validity mismatch")
			}
		})
	}
}

func TestSecp256k1Signer_ErrorCases(t *testing.T) {
	t.Run("Initialize with nil keys", func(t *testing.T) {
		_, err := NewSecp256k1SignerWithKeys(nil, nil)
		assert.Error(t, err, "Should fail to initialize Secp256k1Signer with nil keys")
	})

	t.Run("Signing with nil private key", func(t *testing.T) {
		signer, err := NewSecp256k1Signer()
		require.NoError(t, err, "Failed to initialize Secp256k1Signer")
		signer.keyPair.PrivateKey = nil
		_, err = signer.Sign([]byte("data"))
		assert.Error(t, err, "Signing should fail when private key is nil")
	})

	t.Run("Verifying with nil public key", func(t *testing.T) {
		signer, err := NewSecp256k1Signer()
		require.NoError(t, err, "Failed to initialize Secp256k1Signer")
		signer.keyPair.PublicKey = nil
		valid, err := signer.Verify([]byte("data"), []byte("signature"))
		assert.Error(t, err, "Verification should fail when public key is nil")
		assert.False(t, valid, "Verification should return false when public key is nil")
	})

	t.Run("Signing empty data", func(t *testing.T) {
		signer, err := NewSecp256k1Signer()
		require.NoError(t, err, "Failed to initialize Secp256k1Signer")
		_, err = signer.Sign([]byte{})
		assert.Error(t, err, "Signing should fail with empty data")
	})
}

func TestSecp256k1KeyPair_Serialization(t *testing.T) {
	signer, err := NewSecp256k1Signer()
	require.NoError(t, err, "Failed to initialize Secp256k1Signer")

	privateKeyBytes, err := signer.keyPair.SerializePrivate()
	require.NoError(t, err, "Failed to serialize private key")
	assert.Len(t, privateKeyBytes, 32, "Private key should be 32 bytes long")

	publicKeyBytes, err := signer.keyPair.SerializePublic()
	require.NoError(t, err, "Failed to serialize public key")
	assert.Len(t, publicKeyBytes, 65, "Uncompressed public key should be 65 bytes long") // Updated to 65 bytes

	newSigner, err := NewSecp256k1SignerWithKeys(privateKeyBytes, publicKeyBytes)
	require.NoError(t, err, "Failed to initialize Secp256k1Signer with serialized keys")

	// Verify that the new signer can sign and verify correctly
	data := []byte("Test message for serialization")
	signature, err := newSigner.Sign(data)
	require.NoError(t, err, "Failed to sign data with new signer")

	valid, err := newSigner.Verify(data, signature)
	require.NoError(t, err, "Failed to verify signature with new signer")
	assert.True(t, valid, "Signature should be valid")
}

func TestSecp256k1KeyPair_DeserializeInvalidData(t *testing.T) {
	signer := &Secp256k1KeyPair{}

	// Test deserializing invalid private key data
	err := signer.DeserializePrivate([]byte("short"))
	assert.Error(t, err, "Should fail to deserialize invalid private key data")

	// Test deserializing invalid public key data
	err = signer.DeserializePublic([]byte("short"))
	assert.Error(t, err, "Should fail to deserialize invalid public key data")

	// Test deserializing malformed public key data
	malformedPubKey := make([]byte, 64)
	copy(malformedPubKey, []byte("thisisnotavalidpublickeybytesforsecp256k1"))
	err = signer.DeserializePublic(malformedPubKey)
	assert.Error(t, err, "Should fail to deserialize malformed public key data")
}

func TestSecp256k1Signer_RecoverPublicKey(t *testing.T) {
	signer, err := NewSecp256k1Signer()
	require.NoError(t, err, "Failed to initialize Secp256k1Signer")

	data := []byte("Recoverable signature test message")
	signature, err := signer.Sign(data)
	require.NoError(t, err, "Failed to sign data")

	// Recover the public key from the signature
	hash := crypto.Keccak256(data)
	recoveredPubKey, err := crypto.SigToPub(hash, signature)
	require.NoError(t, err, "Failed to recover public key from signature")

	// Compare the recovered public key with the original public key
	assert.Equal(t, crypto.FromECDSAPub(recoveredPubKey), crypto.FromECDSAPub(signer.keyPair.PublicKey), "Recovered public key should match original public key")
}

func TestSecp256k1Signer_SignatureCompatibility(t *testing.T) {
	// This test ensures that the signature produced is compatible with Ethereum's verification

	signer, err := NewSecp256k1Signer()
	require.NoError(t, err, "Failed to initialize Secp256k1Signer")

	data := []byte("Ethereum compatibility test message")
	signature, err := signer.Sign(data)
	require.NoError(t, err, "Failed to sign data")

	// Ethereum's VerifySignature expects the public key bytes (uncompressed) and R||S
	rS := signature[:64]
	publicKeyBytes := crypto.FromECDSAPub(signer.keyPair.PublicKey)

	valid := crypto.VerifySignature(publicKeyBytes, crypto.Keccak256(data), rS)
	assert.True(t, valid, "Signature should be valid and compatible with Ethereum's verification")
}

func TestSecp256k1Signer_RecoveryID(t *testing.T) {
	// This test ensures that the recovery ID (V) is correctly set to 0 or 1

	signer, err := NewSecp256k1Signer()
	require.NoError(t, err, "Failed to initialize Secp256k1Signer")

	data := []byte("Recovery ID test message")
	signature, err := signer.Sign(data)
	require.NoError(t, err, "Failed to sign data")

	assert.Len(t, signature, 65, "Signature should be 65 bytes long")
	v := signature[64]
	assert.Contains(t, []byte{0, 1}, v, "Recovery ID (V) should be 0 or 1")
}

func TestSecp256k1Signer_EIP155Compatibility(t *testing.T) {
	// Note: Implementing full EIP-155 compatibility requires context of chain ID and transaction signing.
	// This test ensures that signatures can be recovered correctly assuming EIP-155 is handled elsewhere.

	signer, err := NewSecp256k1Signer()
	require.NoError(t, err, "Failed to initialize Secp256k1Signer")

	data := []byte("EIP-155 compatibility test message")
	signature, err := signer.Sign(data)
	require.NoError(t, err, "Failed to sign data")

	// Recover the public key from the signature
	hash := crypto.Keccak256(data)
	recoveredPubKey, err := crypto.SigToPub(hash, signature)
	require.NoError(t, err, "Failed to recover public key from signature")

	// Compare the recovered public key with the original public key
	assert.Equal(t, crypto.FromECDSAPub(recoveredPubKey), crypto.FromECDSAPub(signer.keyPair.PublicKey), "Recovered public key should match original public key")
}
