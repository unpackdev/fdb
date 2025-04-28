// schnorr_test.go

package signatures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchnorrSigner_SignAndVerify(t *testing.T) {
	signer, err := NewSchnorrSigner()
	require.NoError(t, err, "Failed to initialize SchnorrSigner")

	tests := []struct {
		name        string
		data        []byte
		alterData   bool
		expectValid bool
	}{
		{
			name:        "Valid proof",
			data:        []byte("This is a sample message for signing."),
			alterData:   false,
			expectValid: true,
		},
		{
			name:        "Invalid proof with altered data",
			data:        []byte("This is a sample message for signing."),
			alterData:   true,
			expectValid: false,
		},
		{
			name:        "Empty data",
			data:        []byte{},
			alterData:   false,
			expectValid: true, // Depending on your protocol, adjust as needed
		},
		{
			name:        "Nil data",
			data:        nil,
			alterData:   false,
			expectValid: true, // Depending on your protocol, adjust as needed
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

			proof, err := signer.Sign(dataToSign)
			if tt.data == nil {
				dataToSign = []byte{}
			}
			if len(dataToSign) == 0 {
				// Allow empty data to be signed
				require.NoError(t, err, "Failed to generate Schnorr proof for empty data")
			} else {
				require.NoError(t, err, "Failed to generate Schnorr proof")
			}
			assert.NotNil(t, proof, "Proof should not be nil")
			assert.Equal(t, 65, len(proof), "Proof should be 65 bytes long")

			isValid, err := signer.Verify(dataToVerify, proof)
			if tt.expectValid {
				assert.NoError(t, err, "Verification should not error")
				assert.True(t, isValid, "Proof should be valid")
			} else {
				assert.Error(t, err, "Verification should error for invalid proof")
				assert.False(t, isValid, "Proof should be invalid")
			}
		})
	}
}

func TestSchnorrSigner_ErrorCases(t *testing.T) {
	t.Run("Initialize with nil keys", func(t *testing.T) {
		_, err := NewSchnorrSignerWithKeys(nil, nil)
		assert.Error(t, err, "Should fail to initialize SchnorrSigner with nil keys")
	})

	t.Run("Signing with nil private key", func(t *testing.T) {
		signer, err := NewSchnorrSigner()
		require.NoError(t, err, "Failed to initialize SchnorrSigner")
		signer.keyPair.PrivateKey = nil
		_, err = signer.Sign([]byte("data"))
		assert.Error(t, err, "Signing should fail when private key is nil")
	})

	t.Run("Verifying with nil public key", func(t *testing.T) {
		signer, err := NewSchnorrSigner()
		require.NoError(t, err, "Failed to initialize SchnorrSigner")
		signer.keyPair.PublicKey = nil
		valid, err := signer.Verify([]byte("data"), []byte("proof"))
		assert.Error(t, err, "Verification should fail when public key is nil")
		assert.False(t, valid, "Verification should return false when public key is nil")
	})

	t.Run("Verifying with invalid proof", func(t *testing.T) {
		signer, err := NewSchnorrSigner()
		require.NoError(t, err, "Failed to initialize SchnorrSigner")
		valid, err := signer.Verify([]byte("data"), []byte("invalid proof"))
		assert.Error(t, err, "Verification should fail with invalid proof")
		assert.False(t, valid, "Verification should return false with invalid proof")
	})
}

func TestSchnorrKeyPair_Serialization(t *testing.T) {
	signer, err := NewSchnorrSigner()
	require.NoError(t, err, "Failed to initialize SchnorrSigner")

	privateKeyBytes, err := signer.keyPair.SerializePrivate()
	require.NoError(t, err, "Failed to serialize private key")
	assert.NotEmpty(t, privateKeyBytes, "Private key bytes should not be empty")

	publicKeyBytes, err := signer.keyPair.SerializePublic()
	require.NoError(t, err, "Failed to serialize public key")
	assert.NotEmpty(t, publicKeyBytes, "Public key bytes should not be empty")

	newSigner, err := NewSchnorrSignerWithKeys(privateKeyBytes, publicKeyBytes)
	require.NoError(t, err, "Failed to initialize SchnorrSigner with serialized keys")

	data := []byte("Test message")
	proof, err := newSigner.Sign(data)
	require.NoError(t, err, "Failed to create Schnorr proof with new signer")

	valid, err := newSigner.Verify(data, proof)
	require.NoError(t, err, "Failed to verify Schnorr proof with new signer")
	assert.True(t, valid, "Proof should be valid")
}
