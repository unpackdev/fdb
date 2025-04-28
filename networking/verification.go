package networking

import (
	"fmt"
	"github.com/libp2p/go-libp2p/core/crypto"
)

// VerifySignature verifies that the provided signature is valid for the given data and public key.
//
// Parameters:
// - rawPubKey: The raw bytes of the public key.
// - data: The original data that was signed.
// - signature: The signature to verify.
//
// Returns:
// - bool: true if the signature is valid, false otherwise.
// - error: An error if the verification process fails.
func VerifySignature(rawPubKey []byte, data []byte, signature []byte) error {
	// Unmarshal the public key
	pubKey, err := crypto.UnmarshalPublicKey(rawPubKey)
	if err != nil {
		return fmt.Errorf("failed to unmarshal public key: %w", err)
	}

	// Verify the signature
	valid, err := pubKey.Verify(data, signature)
	if err != nil {
		return fmt.Errorf("error during signature verification: %w", err)
	}

	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
