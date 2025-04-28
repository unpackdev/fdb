package share

import "github.com/unpackdev/fdb/types"

type PartialSignature struct {
	Signature  []byte
	ShareIndex int
}

// BaseSigner defines the common interface for signing and verifying data.
type BaseSigner interface {
	Address() (types.Address, error)
	Sign(data []byte) ([]byte, error)
	Verify(data []byte, signature []byte) (bool, error)
	Pair() KeyPair
	Type() types.SignerType
}

type Signer interface {
	BaseSigner
}

type ThresholdSigner interface {
	BaseSigner
	GeneratePartialSignature(data []byte) ([]byte, error)
	VerifyAggregatedSignature(data []byte, signature []byte) (bool, error)
	AggregateAndVerifySignature(blockHash types.Hash, data []byte, partialSigs [][]byte) ([]byte, error)
	AggregatePartialSignatures(blockHash types.Hash, data []byte, partialSigs [][]byte) ([]byte, error)
}

// KeyPair defines the interface for key management.
type KeyPair interface {
	GenerateKey() error
	SerializePrivate() ([]byte, error)
	SerializePublic() ([]byte, error)
	DeserializePrivate(data []byte) error
	DeserializePublic(data []byte) error
	GetPublic() any
	GetPrivate() any
	GetPublicKeyBytes() ([]byte, error)
}
