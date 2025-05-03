// pkg/accounts/utils.go

package accounts

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/unpackdev/fdb/pkg/types"

	"github.com/pkg/errors"
)

// ComputeAccountID computes the account ID using SignerType and public key bytes.
func ComputeAccountID(signerType types.SignerType, pubKeyBytes []byte) string {
	data := append([]byte(signerType.String()), pubKeyBytes...)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// computeAddressFromPublicKey computes the address from the public key bytes.
// It expects the public key bytes to be in the format produced by libp2p's MarshalPublicKey.
// It uses SHA-256 and takes the last AddressSize bytes of the hash.
func computeAddressFromPublicKey(publicKeyBytes []byte) (types.Address, error) {
	var addr types.Address

	if len(publicKeyBytes) == 0 {
		return addr, errors.New("public key is empty")
	}

	// Compute SHA-256 hash of the marshaled public key bytes
	hash := sha256.Sum256(publicKeyBytes)

	// Take the last AddressSize bytes
	copy(addr[:], hash[len(hash)-types.AddressSize:])

	return addr, nil
}

// PublicKeyToAddress derives the Address from the public key bytes.
func PublicKeyToAddress(pubKeyBytes []byte) (types.Address, error) {
	return computeAddressFromPublicKey(pubKeyBytes)
}
