package signatures

import (
	"crypto/sha256"
	"fmt"
	"github.com/peerdns/peerd/pkg/share"
	"github.com/unpackdev/fdb/types"

	"github.com/pkg/errors"
	"sync"

	"github.com/herumi/bls-go-binary/bls"
)

var (
	blsInitOnce sync.Once
	blsInitErr  error
)

// Initialize a sync.Pool for bls.Sign objects to reuse them during verification
var signPool = sync.Pool{
	New: func() interface{} { return new(bls.Sign) },
}

// InitBLS initializes the BLS library.
func InitBLS() error {
	blsInitOnce.Do(func() {
		blsInitErr = bls.Init(bls.BLS12_381)
	})
	return blsInitErr
}

// BLSKeyPair represents a BLS key pair.
type BLSKeyPair struct {
	PrivateKey *bls.SecretKey
	PublicKey  *bls.PublicKey
}

// GenerateKey generates a new BLS key pair.
func (kp *BLSKeyPair) GenerateKey() error {
	if err := InitBLS(); err != nil {
		return fmt.Errorf("failed to initialize BLS library: %w", err)
	}

	kp.PrivateKey = new(bls.SecretKey)
	kp.PrivateKey.SetByCSPRNG()
	kp.PublicKey = kp.PrivateKey.GetPublicKey()
	return nil
}

func (kp *BLSKeyPair) Address() (types.Address, error) {
	return types.ZeroAddress, errors.New("bls signer account address is not yet implemented")
}

// SerializePrivate serializes the private key to bytes.
func (kp *BLSKeyPair) SerializePrivate() ([]byte, error) {
	if kp.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return kp.PrivateKey.Serialize(), nil
}

// SerializePublic serializes the public key to bytes.
func (kp *BLSKeyPair) SerializePublic() ([]byte, error) {
	if kp.PublicKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	return kp.PublicKey.Serialize(), nil
}

// DeserializePrivate deserializes bytes into the private key.
func (kp *BLSKeyPair) DeserializePrivate(data []byte) error {
	if data == nil || len(data) == 0 {
		return fmt.Errorf("private key data is empty")
	}
	if kp.PrivateKey == nil {
		kp.PrivateKey = new(bls.SecretKey)
	}
	if err := kp.PrivateKey.Deserialize(data); err != nil {
		return fmt.Errorf("failed to deserialize private key: %w", err)
	}
	kp.PublicKey = kp.PrivateKey.GetPublicKey()
	return nil
}

// DeserializePublic deserializes bytes into the public key.
func (kp *BLSKeyPair) DeserializePublic(data []byte) error {
	if data == nil || len(data) == 0 {
		return fmt.Errorf("public key data is empty")
	}
	if kp.PublicKey == nil {
		kp.PublicKey = new(bls.PublicKey)
	}
	if err := kp.PublicKey.Deserialize(data); err != nil {
		return fmt.Errorf("failed to deserialize public key: %w", err)
	}
	return nil
}

// GetPublic returns the public key.
func (kp *BLSKeyPair) GetPublic() any {
	return kp.PublicKey
}

func (kp *BLSKeyPair) GetPrivate() any {
	return kp.PrivateKey
}

func (kp *BLSKeyPair) GetPublicKeyBytes() ([]byte, error) {
	if kp.PublicKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}

	return kp.PublicKey.Serialize(), nil
}

// BLSSigner implements the Signer interface using BLS signatures.
type BLSSigner struct {
	keyPair *BLSKeyPair
}

// NewBLSSigner creates a new BLSSigner with a generated key pair.
func NewBLSSigner() (*BLSSigner, error) {
	kp := &BLSKeyPair{}
	if err := kp.GenerateKey(); err != nil {
		return nil, err
	}
	return &BLSSigner{keyPair: kp}, nil
}

// NewBLSSignerWithKeys creates a BLSSigner using provided serialized keys.
func NewBLSSignerWithKeys(privateKeyData, publicKeyData []byte) (*BLSSigner, error) {
	kp := &BLSKeyPair{}
	if err := kp.DeserializePrivate(privateKeyData); err != nil {
		return nil, err
	}
	if err := kp.DeserializePublic(publicKeyData); err != nil {
		return nil, err
	}
	return &BLSSigner{keyPair: kp}, nil
}

func (b *BLSSigner) Address() (types.Address, error) {
	return b.keyPair.Address()
}

func (b *BLSSigner) Pair() share.KeyPair {
	return b.keyPair
}

func (b *BLSSigner) Type() types.SignerType {
	return types.BlsSignerType
}

// Sign signs the given data using BLS.
func (b *BLSSigner) Sign(data []byte) ([]byte, error) {
	if b.keyPair.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	if data == nil || len(data) == 0 {
		return nil, fmt.Errorf("data to sign is empty")
	}
	hash := sha256.Sum256(data)
	sig := b.keyPair.PrivateKey.SignHash(hash[:])
	return sig.Serialize(), nil
}

// Verify verifies the BLS signature for the given data.
func (b *BLSSigner) Verify(data []byte, signature []byte) (bool, error) {
	if b.keyPair.PublicKey == nil {
		return false, fmt.Errorf("public key is nil")
	}
	if data == nil || len(data) == 0 {
		return false, fmt.Errorf("data to verify is empty")
	}
	if signature == nil || len(signature) == 0 {
		return false, fmt.Errorf("signature is empty")
	}
	var sig bls.Sign
	if err := sig.Deserialize(signature); err != nil {
		return false, fmt.Errorf("failed to deserialize signature: %w", err)
	}
	hash := sha256.Sum256(data)
	return sig.VerifyHash(b.keyPair.PublicKey, hash[:]), nil
}

// AggregateSignatures aggregates multiple BLS signatures into a single signature.
func AggregateSignatures(signatures [][]byte) ([]byte, error) {
	if len(signatures) == 0 {
		return nil, fmt.Errorf("no signatures to aggregate")
	}
	var aggSig bls.Sign
	for _, sigBytes := range signatures {
		if len(sigBytes) == 0 {
			return nil, fmt.Errorf("one of the signatures is empty")
		}
		var s bls.Sign
		if err := s.Deserialize(sigBytes); err != nil {
			return nil, fmt.Errorf("failed to deserialize signature: %w", err)
		}
		aggSig.Add(&s)
	}
	return aggSig.Serialize(), nil
}

// DeserializePublic initializes the BLSSigner with a public key.
func (b *BLSSigner) DeserializePublic(publicKeyBytes []byte) error {
	if b.keyPair == nil {
		b.keyPair = &BLSKeyPair{}
	}

	return b.keyPair.DeserializePublic(publicKeyBytes)
}
