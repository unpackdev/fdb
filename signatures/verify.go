package signatures

import (
	"fmt"
	"github.com/unpackdev/fdb/types"
)

// Verify verifies the signature based on the SignerType and public key.
// It returns nil if the signature is valid; otherwise, it returns an error.
func Verify(sType types.SignerType, publicKeyBytes []byte, data, signature []byte) error {
	// Retrieve the appropriate Signer from the registry
	signer, err := GetSignerByType(sType, publicKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to get signer: %w", err)
	}

	// Perform verification using the signer
	valid, err := signer.Verify(data, signature)
	if err != nil {
		return fmt.Errorf("verification error: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid signature")
	}

	return nil
}
