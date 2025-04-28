package signatures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBLSSigner_SignAndVerify(t *testing.T) {
	signer, err := NewBLSSigner()
	require.NoError(t, err, "Failed to initialize BLSSigner")

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

func TestBLSSigner_AggregateSignatures(t *testing.T) {
	// Initialize multiple BLSSigners
	signers := make([]*BLSSigner, 5)
	for i := 0; i < len(signers); i++ {
		signer, err := NewBLSSigner()
		require.NoError(t, err, "Failed to initialize BLSSigner")
		signers[i] = signer
	}

	// Sample data to sign
	data := []byte("Common message for signing.")

	// Each signer signs the same data
	signatures := make([][]byte, len(signers))
	for i, signer := range signers {
		sig, err := signer.Sign(data)
		require.NoError(t, err, "Failed to sign data")
		signatures[i] = sig
	}

	// Aggregate the signatures
	aggregatedSignature, err := AggregateSignatures(signatures)
	assert.NoError(t, err, "Failed to aggregate signatures")
	assert.NotNil(t, aggregatedSignature, "Aggregated signature should not be nil")

	// Note: Proper verification of aggregated signatures requires aggregated public keys and is beyond the scope of this test.
}

func TestBLSSigner_ErrorCases(t *testing.T) {
	t.Run("Initialize with nil keys", func(t *testing.T) {
		_, err := NewBLSSignerWithKeys(nil, nil)
		assert.Error(t, err, "Should fail to initialize BLSSigner with nil keys")
	})

	t.Run("Signing with nil private key", func(t *testing.T) {
		signer, err := NewBLSSigner()
		require.NoError(t, err, "Failed to initialize BLSSigner")
		signer.keyPair.PrivateKey = nil
		_, err = signer.Sign([]byte("data"))
		assert.Error(t, err, "Signing should fail when private key is nil")
	})

	t.Run("Verifying with nil public key", func(t *testing.T) {
		signer, err := NewBLSSigner()
		require.NoError(t, err, "Failed to initialize BLSSigner")
		signer.keyPair.PublicKey = nil
		valid, err := signer.Verify([]byte("data"), []byte("signature"))
		assert.Error(t, err, "Verification should fail when public key is nil")
		assert.False(t, valid, "Verification should return false when public key is nil")
	})

	t.Run("Signing empty data", func(t *testing.T) {
		signer, err := NewBLSSigner()
		require.NoError(t, err, "Failed to initialize BLSSigner")
		_, err = signer.Sign([]byte{})
		assert.Error(t, err, "Signing should fail with empty data")
	})
}

func TestBLSKeySerialization(t *testing.T) {
	signer, err := NewBLSSigner()
	require.NoError(t, err, "Failed to initialize BLSSigner")

	privateKeyBytes, err := signer.keyPair.SerializePrivate()
	require.NoError(t, err, "Failed to serialize private key")
	assert.NotEmpty(t, privateKeyBytes, "Private key bytes should not be empty")

	publicKeyBytes, err := signer.keyPair.SerializePublic()
	require.NoError(t, err, "Failed to serialize public key")
	assert.NotEmpty(t, publicKeyBytes, "Public key bytes should not be empty")

	newSigner, err := NewBLSSignerWithKeys(privateKeyBytes, publicKeyBytes)
	require.NoError(t, err, "Failed to initialize BLSSigner with serialized keys")

	data := []byte("Test message")
	signature, err := newSigner.Sign(data)
	require.NoError(t, err, "Failed to sign data with new signer")

	valid, err := newSigner.Verify(data, signature)
	require.NoError(t, err, "Failed to verify signature with new signer")
	assert.True(t, valid, "Signature should be valid")
}

func TestAggregateSignatures_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		signatures    [][]byte
		expectError   bool
		expectedError string
	}{
		{
			name:          "No signatures",
			signatures:    [][]byte{},
			expectError:   true,
			expectedError: "no signatures to aggregate",
		},
		{
			name:          "Empty signature",
			signatures:    [][]byte{[]byte{}},
			expectError:   true,
			expectedError: "one of the signatures is empty",
		},
		{
			name:          "Invalid signature data",
			signatures:    [][]byte{[]byte("invalid")},
			expectError:   true,
			expectedError: "failed to deserialize signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregatedSignature, err := AggregateSignatures(tt.signatures)
			if tt.expectError {
				assert.Error(t, err, "Expected an error")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message mismatch")
				assert.Nil(t, aggregatedSignature, "Aggregated signature should be nil when error occurs")
			} else {
				assert.NoError(t, err, "Did not expect an error")
				assert.NotNil(t, aggregatedSignature, "Aggregated signature should not be nil")
			}
		})
	}
}
