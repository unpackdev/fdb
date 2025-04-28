package signatures

import (
	"fmt"

	"go.dedis.ch/kyber/v4/pairing/bls12381/kilic"
	"go.dedis.ch/kyber/v4/pairing/bn256"
	dkg "go.dedis.ch/kyber/v4/share/dkg/pedersen"
)

// SuiteType represents the type of cryptographic suite.
type SuiteType string

const (
	// BN256 represents the BN256 pairing suite.
	BN256 SuiteType = "BN256"
	// BLS12381 represents the BLS12-381 pairing suite.
	BLS12381 SuiteType = "BLS12381"
)

// NewSuiteFactory returns a dkg.Suite based on the provided SuiteType.
func NewSuiteFactory(suiteType SuiteType) (dkg.Suite, error) {
	switch suiteType {
	case BN256:
		return bn256.NewSuiteG2(), nil
	case BLS12381:
		return bls12381.NewSuiteG2(), nil
	default:
		return nil, fmt.Errorf("unsupported suite type: %s", suiteType)
	}
}
