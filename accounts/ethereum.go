// pkg/accounts/ethereum.go
package accounts

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/pkg/errors"
)

// computeEthereumAddress computes the Ethereum address from the libp2p public key.
// It uses Keccak-256 hashing and returns the address in hexadecimal format with '0x' prefix.
// computeEthereumAddress computes the Ethereum address from the libp2p public key.
func computeEthereumAddress(pubKey libp2pCrypto.PubKey) (common.Address, error) {
	var pubKeyBytes []byte
	var err error

	switch pubKey.Type() {
	case libp2pCrypto.Secp256k1:
		// Get the raw public key bytes
		rawPubKeyBytes, err := pubKey.Raw()
		if err != nil {
			return common.Address{}, errors.Wrap(err, "failed to get raw public key bytes")
		}

		if len(rawPubKeyBytes) == 33 {
			// Compressed key, decompress it
			pubKeyDecompressed, err := ethCrypto.DecompressPubkey(rawPubKeyBytes)
			if err != nil {
				return common.Address{}, errors.Wrap(err, "failed to decompress Secp256k1 public key")
			}
			pubKeyBytes = ethCrypto.FromECDSAPub(pubKeyDecompressed)[1:] // Remove 0x04 prefix
		} else if len(rawPubKeyBytes) == 65 {
			// Uncompressed key, remove the prefix
			pubKeyBytes = rawPubKeyBytes[1:] // Remove 0x04 prefix
		} else {
			return common.Address{}, errors.Errorf("unexpected Secp256k1 public key length: got %d bytes", len(rawPubKeyBytes))
		}
	case libp2pCrypto.Ed25519:
		// Use the raw public key bytes for Ed25519 keys
		pubKeyBytes, err = pubKey.Raw()
		if err != nil {
			return common.Address{}, errors.Wrap(err, "failed to get raw Ed25519 public key bytes")
		}
	default:
		return common.Address{}, errors.Errorf("unsupported key type: %d", pubKey.Type())
	}

	// Compute Keccak-256 hash of the public key bytes
	hash := ethCrypto.Keccak256(pubKeyBytes)

	// Take the last 20 bytes for the Ethereum address
	ethAddress := common.BytesToAddress(hash[12:])

	return ethAddress, nil
}

// computeEthereumAddressFromECDSAPubKey computes the Ethereum address from an ECDSA public key.
// It uses Keccak-256 hashing and returns the address in hexadecimal format with '0x' prefix.
func computeEthereumAddressFromECDSAPubKey(pubKey *ecdsa.PublicKey) (common.Address, error) {
	if pubKey == nil {
		return common.Address{}, errors.New("public key is nil")
	}

	pubBytes := ethCrypto.FromECDSAPub(pubKey) // 65 bytes with 0x04 prefix
	hash := ethCrypto.Keccak256(pubBytes[1:])  // Exclude the 0x04 prefix

	return common.BytesToAddress(hash[12:]), nil // Last 20 bytes
}
