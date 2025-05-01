package networking

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/unpackdev/fdb/logger"
	"go.uber.org/zap"
)

// VerifySignature verifies that the provided signature is valid for the given data and public key.
//
// Parameters:
// - rawPubKey: The raw bytes of the public key.
// - data: The original data that was signed.
// - signature: The signature to verify.
//
// Returns:
// - error: An error if the verification process fails, nil if successful.
func VerifySignature(rawPubKey []byte, data []byte, signature []byte) error {
	logger := logger.G()

	// Log the input parameters for debugging
	logger.Debug("Starting signature verification",
		zap.Int("public_key_size", len(rawPubKey)),
		zap.Int("data_size", len(data)),
		zap.Int("signature_size", len(signature)))

	// Check for empty or obviously invalid inputs
	if len(rawPubKey) == 0 {
		logger.Error("Empty public key provided")
		return fmt.Errorf("empty public key")
	}

	if len(signature) == 0 {
		logger.Error("Empty signature provided")
		return fmt.Errorf("empty signature")
	}

	if len(data) == 0 {
		logger.Error("Empty data provided")
		return fmt.Errorf("empty data")
	}

	// Log the first few bytes of each for inspection
	pubKeyPrefix := rawPubKey
	if len(pubKeyPrefix) > 16 {
		pubKeyPrefix = pubKeyPrefix[:16]
	}

	logger.Debug("Public key prefix",
		zap.Binary("prefix", pubKeyPrefix))

	// Log if data is very large
	if len(data) > 50*1024 {
		logger.Warn("Verifying signature for very large data",
			zap.Int("data_size_kb", len(data)/1024))
	}

	// Unmarshal the public key
	logger.Debug("Attempting to unmarshal public key")
	pubKey, err := crypto.UnmarshalPublicKey(rawPubKey)
	if err != nil {
		logger.Error("Failed to unmarshal public key",
			zap.Error(err),
			zap.Int("key_size", len(rawPubKey)))
		return fmt.Errorf("failed to unmarshal public key: %w", err)
	}

	logger.Debug("Successfully unmarshaled public key",
		zap.String("key_type", pubKey.Type().String()))

	// Verify the signature
	logger.Debug("Starting signature verification")
	valid, err := pubKey.Verify(data, signature)
	if err != nil {
		logger.Error("Error during signature verification",
			zap.Error(err))
		return fmt.Errorf("error during signature verification: %w", err)
	}

	if !valid {
		logger.Error("Invalid signature")
		return fmt.Errorf("invalid signature")
	}

	logger.Debug("Signature verified successfully")
	return nil
}
