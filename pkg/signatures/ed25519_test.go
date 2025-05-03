package signatures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEd25519Signer_SignAndVerify(t *testing.T) {
	signer, err := NewEd25519Signer()
	require.NoError(t, err, "Failed to initialize Ed25519Signer")

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataToSign := tt.data
			dataToVerify := tt.data

			if tt.alterData && len(tt.data) > 0 {
				// Create a copy of the data and alter it for verification
				dataToVerify = append([]byte{}, tt.data...)
				dataToVerify[0] ^= 0xFF // Alter the data
			}

			signature, err := signer.Sign(dataToSign)
			if tt.expectSignErr {
				assert.Error(t, err, "Expected signing to fail")
				return
			} else {
				assert.NoError(t, err, "Expected signing to succeed")
				assert.NotNil(t, signature, "Signature should not be nil")
			}

			isValid, err := signer.Verify(dataToVerify, signature)
			if tt.expectVerifyErr {
				assert.Error(t, err, "Expected verification to error")
			} else {
				assert.NoError(t, err, "Verification should not error")
			}
			assert.Equal(t, tt.expectValid, isValid, "Signature validity mismatch")
		})
	}
}

func TestEd25519Signer_ErrorCases(t *testing.T) {
	t.Run("Initialize with nil keys", func(t *testing.T) {
		_, err := NewEd25519SignerWithKeys(nil, nil)
		assert.Error(t, err, "Should fail to initialize Ed25519Signer with nil keys")
	})

	t.Run("Signing with nil private key", func(t *testing.T) {
		signer, err := NewEd25519Signer()
		require.NoError(t, err, "Failed to initialize Ed25519Signer")
		signer.keyPair.PrivateKey = nil
		_, err = signer.Sign([]byte("data"))
		assert.Error(t, err, "Signing should fail when private key is nil")
	})

	t.Run("Verifying with nil public key", func(t *testing.T) {
		signer, err := NewEd25519Signer()
		require.NoError(t, err, "Failed to initialize Ed25519Signer")
		signer.keyPair.PublicKey = nil
		valid, err := signer.Verify([]byte("data"), []byte("signature"))
		assert.Error(t, err, "Verification should fail when public key is nil")
		assert.False(t, valid, "Verification should return false when public key is nil")
	})

	t.Run("Signing empty data", func(t *testing.T) {
		signer, err := NewEd25519Signer()
		require.NoError(t, err, "Failed to initialize Ed25519Signer")
		_, err = signer.Sign([]byte{})
		assert.Error(t, err, "Signing should fail with empty data")
	})
}

func TestEd25519KeySerialization(t *testing.T) {
	signer, err := NewEd25519Signer()
	require.NoError(t, err, "Failed to initialize Ed25519Signer")

	privateKeyBytes, err := signer.keyPair.SerializePrivate()
	require.NoError(t, err, "Failed to serialize private key")
	assert.NotEmpty(t, privateKeyBytes, "Private key bytes should not be empty")

	publicKeyBytes, err := signer.keyPair.SerializePublic()
	require.NoError(t, err, "Failed to serialize public key")
	assert.NotEmpty(t, publicKeyBytes, "Public key bytes should not be empty")

	newSigner, err := NewEd25519SignerWithKeys(privateKeyBytes, publicKeyBytes)
	require.NoError(t, err, "Failed to initialize Ed25519Signer with serialized keys")

	data := []byte("Test message")
	signature, err := newSigner.Sign(data)
	require.NoError(t, err, "Failed to sign data with new signer")

	valid, err := newSigner.Verify(data, signature)
	require.NoError(t, err, "Failed to verify signature with new signer")
	assert.True(t, valid, "Signature should be valid")
}
