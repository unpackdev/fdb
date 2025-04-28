// schnorr.go

package signatures

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/peerdns/peerd/pkg/share"
	"github.com/unpackdev/fdb/types"

	"github.com/cloudflare/circl/group"
)

// SchnorrKeyPair represents a Schnorr key pair.
type SchnorrKeyPair struct {
	PrivateKey group.Scalar
	PublicKey  group.Element
	Params     group.Group
}

// GenerateKey generates a new Schnorr key pair.
func (kp *SchnorrKeyPair) GenerateKey() error {
	if kp.Params == nil {
		return fmt.Errorf("group parameters are not set")
	}

	// Generate private key: random scalar
	kp.PrivateKey = kp.Params.RandomScalar(rand.Reader)

	// Compute public key: g^x
	g := kp.Params.Generator()
	kp.PublicKey = kp.Params.NewElement().Mul(g, kp.PrivateKey)

	return nil
}

// SerializePrivate serializes the private key to bytes.
func (kp *SchnorrKeyPair) SerializePrivate() ([]byte, error) {
	if kp.PrivateKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return kp.PrivateKey.MarshalBinary()
}

// SerializePublic serializes the public key to bytes.
func (kp *SchnorrKeyPair) SerializePublic() ([]byte, error) {
	if kp.PublicKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	return kp.PublicKey.MarshalBinary()
}

// DeserializePrivate deserializes bytes into the private key.
func (kp *SchnorrKeyPair) DeserializePrivate(data []byte) error {
	if kp.Params == nil {
		return fmt.Errorf("group parameters are not set")
	}
	if len(data) == 0 {
		return fmt.Errorf("private key data is empty")
	}
	scalar := kp.Params.NewScalar()
	if err := scalar.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("failed to unmarshal private key: %w", err)
	}
	kp.PrivateKey = scalar

	// Compute public key from private key
	g := kp.Params.Generator()
	kp.PublicKey = kp.Params.NewElement().Mul(g, kp.PrivateKey)
	return nil
}

// DeserializePublic deserializes bytes into the public key.
func (kp *SchnorrKeyPair) DeserializePublic(data []byte) error {
	if kp.Params == nil {
		return fmt.Errorf("group parameters are not set")
	}
	if len(data) == 0 {
		return fmt.Errorf("public key data is empty")
	}
	elem := kp.Params.NewElement()
	if err := elem.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("failed to unmarshal public key: %w", err)
	}
	kp.PublicKey = elem
	return nil
}

func (kp *SchnorrKeyPair) Address() (types.Address, error) {
	return types.ZeroAddress, errors.New("schnorr signer account address is not yet implemented")
}

// GetPublic returns the public key.
func (kp *SchnorrKeyPair) GetPublic() any {
	return kp.PublicKey
}

func (kp *SchnorrKeyPair) GetPrivate() any {
	return kp.PrivateKey
}

func (kp *SchnorrKeyPair) GetPublicKeyBytes() ([]byte, error) {
	return kp.PublicKey.MarshalBinary()
}

// SchnorrSigner implements the Signer interface using Schnorr-based ZK proofs.
type SchnorrSigner struct {
	keyPair *SchnorrKeyPair
}

// NewSchnorrSigner creates a new SchnorrSigner with a generated key pair.
func NewSchnorrSigner() (*SchnorrSigner, error) {
	// Initialize group parameters, e.g., P256
	params := group.P256

	kp := &SchnorrKeyPair{
		Params: params,
	}

	if err := kp.GenerateKey(); err != nil {
		return nil, err
	}

	return &SchnorrSigner{keyPair: kp}, nil
}

// NewSchnorrSignerWithKeys creates a SchnorrSigner using provided serialized keys.
func NewSchnorrSignerWithKeys(privateKeyData, publicKeyData []byte) (*SchnorrSigner, error) {
	params := group.P256

	kp := &SchnorrKeyPair{
		Params: params,
	}

	if err := kp.DeserializePrivate(privateKeyData); err != nil {
		return nil, err
	}

	if err := kp.DeserializePublic(publicKeyData); err != nil {
		return nil, err
	}

	return &SchnorrSigner{keyPair: kp}, nil
}

// Pair returns the SchnorrKeyPair.
func (s *SchnorrSigner) Pair() share.KeyPair {
	return s.keyPair
}

// Type returns the SignerType.
func (s *SchnorrSigner) Type() types.SignerType {
	return types.SchnorrSignerType
}

func (s *SchnorrSigner) Address() (types.Address, error) {
	return s.keyPair.Address()
}

// Sign generates a Schnorr-based ZK proof for the given data.
func (s *SchnorrSigner) Sign(data []byte) ([]byte, error) {
	if s.keyPair.PrivateKey == nil || s.keyPair.PublicKey == nil {
		return nil, fmt.Errorf("keys are not initialized")
	}

	// Treat nil data as empty slice
	if data == nil {
		data = []byte{}
	}

	// Compute the hash of the data
	hash := sha256.Sum256(data)

	// Create commitment: C = g^k for random k
	k := s.keyPair.Params.RandomScalar(rand.Reader)
	C := s.keyPair.Params.NewElement().Mul(s.keyPair.Params.Generator(), k)

	// Compute challenge: e = H(C || y || hash)
	challengeHash := sha256.New()
	CBytes, err := C.MarshalBinaryCompress()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal commitment: %w", err)
	}
	challengeHash.Write(CBytes)

	yBytes, err := s.keyPair.PublicKey.MarshalBinaryCompress()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	challengeHash.Write(yBytes)

	challengeHash.Write(hash[:])
	eBytes := challengeHash.Sum(nil)

	// Create group.Scalar from eBytes
	e := s.keyPair.Params.NewScalar()
	if err := e.UnmarshalBinary(eBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal challenge scalar: %w", err)
	}

	// Compute response: s = k + e * x mod q
	ex := s.keyPair.Params.NewScalar().Mul(e, s.keyPair.PrivateKey)
	sVal := s.keyPair.Params.NewScalar().Add(k, ex)

	// Serialize the proof: C || s
	proofBytes, err := C.MarshalBinaryCompress()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal commitment: %w", err)
	}
	sBytes, err := sVal.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response scalar: %w", err)
	}

	proof := append(proofBytes, sBytes...)

	return proof, nil
}

// Verify verifies the Schnorr-based ZK proof.
func (s *SchnorrSigner) Verify(data []byte, proof []byte) (bool, error) {
	if s.keyPair.PublicKey == nil {
		return false, fmt.Errorf("public key is not initialized")
	}

	// Treat nil data as empty slice
	if data == nil {
		data = []byte{}
	}

	// For P256, commitment C is 33 bytes (compressed), and s is 32 bytes
	const commitmentSize = 33
	const sValSize = 32
	expectedProofSize := commitmentSize + sValSize
	if len(proof) != expectedProofSize {
		return false, fmt.Errorf("invalid proof length: expected %d, got %d", expectedProofSize, len(proof))
	}

	// Extract C and s from the proof
	CBytes := proof[:commitmentSize]
	sBytes := proof[commitmentSize:]

	sVal := s.keyPair.Params.NewScalar()
	if err := sVal.UnmarshalBinary(sBytes); err != nil {
		return false, fmt.Errorf("failed to unmarshal response scalar: %w", err)
	}

	C := s.keyPair.Params.NewElement()
	if err := C.UnmarshalBinary(CBytes); err != nil {
		return false, fmt.Errorf("failed to unmarshal commitment: %w", err)
	}

	// Compute e = H(C || y || hash)
	hashData := sha256.Sum256(data)
	challengeHash := sha256.New()
	challengeHash.Write(CBytes)

	yBytes, err := s.keyPair.PublicKey.MarshalBinaryCompress()
	if err != nil {
		return false, fmt.Errorf("failed to marshal public key: %w", err)
	}
	challengeHash.Write(yBytes)

	challengeHash.Write(hashData[:])
	eBytes := challengeHash.Sum(nil)

	e := s.keyPair.Params.NewScalar()
	if err := e.UnmarshalBinary(eBytes); err != nil {
		return false, fmt.Errorf("failed to unmarshal challenge scalar: %w", err)
	}

	// Compute g^s
	gS := s.keyPair.Params.NewElement().Mul(s.keyPair.Params.Generator(), sVal)

	// Compute C + y^e
	yE := s.keyPair.Params.NewElement().Mul(s.keyPair.PublicKey, e)
	CyE := s.keyPair.Params.NewElement().Add(C, yE)

	// Check if g^s == C + y^e
	if gS.IsEqual(CyE) {
		return true, nil
	}

	return false, fmt.Errorf("invalid proof")
}

// DeserializePublic initializes the SchnorrSigner with a public key.
func (s *SchnorrSigner) DeserializePublic(publicKeyBytes []byte) error {
	if s.keyPair == nil {
		s.keyPair = &SchnorrKeyPair{
			Params: group.P256,
		}
	}
	return s.keyPair.DeserializePublic(publicKeyBytes)
}
