package signatures

import (
	"fmt"
	"github.com/unpackdev/fdb/share"
	"github.com/unpackdev/fdb/types"

	"github.com/sasha-s/go-deadlock"
)

// RegisterSignerFunc defines a function type for registering signers.
type RegisterSignerFunc func(publicKeyBytes []byte) (share.Signer, error)

// signerRegistry maintains a mapping from SignerType to Signer constructor functions.
var (
	signerRegistry   = make(map[types.SignerType]RegisterSignerFunc)
	signerRegistryMu deadlock.RWMutex
)

// RegisterSigner registers a signer constructor for a specific SignerType.
func RegisterSigner(sType types.SignerType, constructor RegisterSignerFunc) {
	signerRegistryMu.Lock()
	defer signerRegistryMu.Unlock()
	signerRegistry[sType] = constructor
}

// GetSignerByType retrieves a Signer instance based on SignerType and initializes it with the provided public key.
func GetSignerByType(sType types.SignerType, publicKeyBytes []byte) (share.Signer, error) {
	signerRegistryMu.RLock()
	constructor, exists := signerRegistry[sType]
	signerRegistryMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no signer registered for type: %v", sType)
	}

	return constructor(publicKeyBytes)
}

// Initialization to register all existing signers.
func init() {
	RegisterSigner(types.BlsSignerType, func(publicKeyBytes []byte) (share.Signer, error) {
		signer := &BLSSigner{}
		if err := signer.DeserializePublic(publicKeyBytes); err != nil {
			return nil, fmt.Errorf("failed to deserialize BLS public key: %w", err)
		}
		return signer, nil
	})

	RegisterSigner(types.Ed25519SignerType, func(publicKeyBytes []byte) (share.Signer, error) {
		signer := &Ed25519Signer{}
		if err := signer.DeserializePublic(publicKeyBytes); err != nil {
			return nil, fmt.Errorf("failed to deserialize Ed25519 public key: %w", err)
		}
		return signer, nil
	})

	// Register SchnorrSignerType
	RegisterSigner(types.SchnorrSignerType, func(publicKeyBytes []byte) (share.Signer, error) {
		signer := &SchnorrSigner{}
		if err := signer.DeserializePublic(publicKeyBytes); err != nil {
			return nil, fmt.Errorf("failed to deserialize Schnorr public key: %w", err)
		}
		return signer, nil
	})

	RegisterSigner(types.Secp256k1SignerType, func(publicKeyBytes []byte) (share.Signer, error) {
		signer := &Secp256k1Signer{}
		if err := signer.DeserializePublic(publicKeyBytes); err != nil {
			return nil, fmt.Errorf("failed to deserialize Secp256k1 public key: %w", err)
		}
		return signer, nil
	})
}
