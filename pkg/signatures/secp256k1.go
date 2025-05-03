// signatures/secp256k1.go

package signatures

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/unpackdev/fdb/pkg/share"
	"github.com/unpackdev/fdb/pkg/types"
)

// Secp256k1KeyPair represents a secp256k1 ECDSA key pair.
type Secp256k1KeyPair struct {
	address    types.Address
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// GenerateKey generates a new secp256k1 ECDSA key pair.
func (kp *Secp256k1KeyPair) GenerateKey() error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate secp256k1 key pair: %w", err)
	}
	kp.PrivateKey = privateKey
	kp.PublicKey = &privateKey.PublicKey

	// Get the public key bytes
	publicKeyBytes := crypto.FromECDSAPub(kp.PublicKey)
	if len(publicKeyBytes) == 0 {
		return fmt.Errorf("public key is empty")
	}

	// Remove the prefix byte.
	// Ethereum does not use 1st byte which stands for uncompressed indicator to generate the address.
	// The first byte is not part of the actual key material but identifier that signals that key is uncompressed.
	pubBytes := publicKeyBytes[1:]

	// Use Keccak256 hash (for Ethereum compatibility)
	hash := crypto.Keccak256(pubBytes)
	copy(kp.address[:], hash[len(hash)-types.AddressSize:])

	return nil
}

func (kp *Secp256k1KeyPair) Address() (types.Address, error) {
	if kp.address.Equals(types.ZeroAddress) {
		return types.ZeroAddress, fmt.Errorf("address not generated; please generate the key pair first")
	}
	return kp.address, nil
}

// SerializePrivate serializes the private key to bytes.
func (kp *Secp256k1KeyPair) SerializePrivate() ([]byte, error) {
	if kp.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return crypto.FromECDSA(kp.PrivateKey), nil
}

// SerializePublic serializes the public key to bytes.
func (kp *Secp256k1KeyPair) SerializePublic() ([]byte, error) {
	if kp.PublicKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	return crypto.FromECDSAPub(kp.PublicKey), nil
}

// DeserializePrivate deserializes bytes into the private key.
func (kp *Secp256k1KeyPair) DeserializePrivate(data []byte) error {
	if len(data) != 32 {
		return fmt.Errorf("invalid private key data length: expected 32 bytes, got %d", len(data))
	}
	privateKey, err := crypto.ToECDSA(data)
	if err != nil {
		return fmt.Errorf("failed to deserialize private key: %w", err)
	}
	kp.PrivateKey = privateKey
	kp.PublicKey = &privateKey.PublicKey

	// Recompute the address
	publicKeyBytes := crypto.FromECDSAPub(kp.PublicKey)
	if len(publicKeyBytes) == 0 {
		return fmt.Errorf("public key is empty")
	}

	// Remove the prefix byte.
	// Ethereum does not use 1st byte which stands for uncompressed indicator to generate the address.
	// The first byte is not part of the actual key material but identifier that signals that key is uncompressed.
	pubBytes := publicKeyBytes[1:]

	hash := crypto.Keccak256(pubBytes)
	copy(kp.address[:], hash[len(hash)-types.AddressSize:])

	return nil
}

// DeserializePublic deserializes bytes into the public key.
func (kp *Secp256k1KeyPair) DeserializePublic(data []byte) error {
	if len(data) != 64 && len(data) != 65 {
		return fmt.Errorf("invalid public key data length: expected 64 or 65 bytes, got %d", len(data))
	}
	publicKey, err := crypto.UnmarshalPubkey(data)
	if err != nil {
		return fmt.Errorf("failed to deserialize public key: %w", err)
	}
	kp.PublicKey = publicKey
	return nil
}

// GetPublic returns the public key.
func (kp *Secp256k1KeyPair) GetPublic() any {
	return kp.PublicKey
}

func (kp *Secp256k1KeyPair) GetPrivate() any {
	return kp.PrivateKey
}

// GetPublicKeyBytes returns the serialized public key bytes.
func (kp *Secp256k1KeyPair) GetPublicKeyBytes() ([]byte, error) {
	if kp.PublicKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	return crypto.FromECDSAPub(kp.PublicKey), nil
}

// Secp256k1Signer implements the Signer interface using secp256k1 ECDSA signatures.
type Secp256k1Signer struct {
	keyPair *Secp256k1KeyPair
}

// NewSecp256k1Signer creates a new Secp256k1Signer with a generated key pair.
func NewSecp256k1Signer() (*Secp256k1Signer, error) {
	kp := &Secp256k1KeyPair{}
	if err := kp.GenerateKey(); err != nil {
		return nil, err
	}
	return &Secp256k1Signer{keyPair: kp}, nil
}

// NewSecp256k1SignerWithKeys creates a Secp256k1Signer using provided serialized keys.
func NewSecp256k1SignerWithKeys(privateKeyData, publicKeyData []byte) (*Secp256k1Signer, error) {
	kp := &Secp256k1KeyPair{}
	if err := kp.DeserializePrivate(privateKeyData); err != nil {
		return nil, err
	}
	if err := kp.DeserializePublic(publicKeyData); err != nil {
		return nil, err
	}
	return &Secp256k1Signer{keyPair: kp}, nil
}

// Pair returns the underlying key pair.
func (s *Secp256k1Signer) Pair() share.KeyPair {
	return s.keyPair
}

// Type returns the SignerType.
func (s *Secp256k1Signer) Type() types.SignerType {
	return types.Secp256k1SignerType
}

func (s *Secp256k1Signer) Address() (types.Address, error) {
	return s.keyPair.Address()
}

// Sign signs the given data using secp256k1 ECDSA.
// It returns the signature bytes in [R || S || V] format, where V is 0 or 1.
func (s *Secp256k1Signer) Sign(data []byte) ([]byte, error) {
	if s.keyPair.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("data to sign is empty")
	}

	// Sign the data directly
	signature, err := crypto.Sign(data, s.keyPair.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	// Ensure signature length is correct
	if len(signature) != 65 {
		return nil, fmt.Errorf("invalid signature length: expected 65 bytes, got %d", len(signature))
	}

	// Do not adjust V; keep it as 0 or 1
	return signature, nil
}

// Verify verifies the secp256k1 ECDSA signature for the given data.
// It expects the signature to be in [R || S || V] format, where V is 0 or 1.
func (s *Secp256k1Signer) Verify(data []byte, signature []byte) (bool, error) {
	if s.keyPair.PublicKey == nil {
		return false, fmt.Errorf("public key is nil")
	}
	if len(data) == 0 {
		return false, fmt.Errorf("data to verify is empty")
	}
	if len(signature) != 65 {
		return false, fmt.Errorf("invalid signature length: expected 65 bytes, got %d", len(signature))
	}

	// The first 64 bytes are R and S
	sig := signature[:64]

	// Verify the signature directly
	valid := crypto.VerifySignature(crypto.FromECDSAPub(s.keyPair.PublicKey), data, sig)
	return valid, nil
}

// DeserializePublic initializes the Secp256k1Signer with a public key.
func (s *Secp256k1Signer) DeserializePublic(publicKeyBytes []byte) error {
	if s.keyPair == nil {
		s.keyPair = &Secp256k1KeyPair{}
	}
	return s.keyPair.DeserializePublic(publicKeyBytes)
}
