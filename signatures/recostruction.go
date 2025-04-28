// pkg/signatures/reconstruction.go

package signatures

import (
	"fmt"

	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/share"
)

// ReconstructMasterPrivateKey reconstructs the master private key from collected shares using Lagrange interpolation.
func ReconstructMasterPrivateKey(g kyber.Group, shares []*share.PriShare, threshold int) (kyber.Scalar, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("no shares provided for reconstruction")
	}

	// Ensure all shares are from distinct indices
	indexMap := make(map[int]bool)
	for _, s := range shares {
		if _, exists := indexMap[s.I]; exists {
			return nil, fmt.Errorf("duplicate share index detected: %d", s.I)
		}
		indexMap[s.I] = true
	}

	// Perform Lagrange interpolation at 0 to reconstruct the secret
	return share.RecoverSecret(g, shares, threshold, 0)
}

// ReconstructSecret reconstructs the shared secret p(0) from a list of private shares using Lagrange interpolation.
func ReconstructSecret(group kyber.Group, shares []*share.PriShare, t, n int) (kyber.Scalar, error) {
	if len(shares) < t {
		return nil, fmt.Errorf("not enough shares to reconstruct the secret: have %d, need %d", len(shares), t)
	}

	// Lagrange interpolation
	secret := group.Scalar().Zero()

	for i, shareI := range shares[:t] {
		li := group.Scalar().One()
		xi := group.Scalar().SetInt64(int64(shareI.I))
		for j, shareJ := range shares[:t] {
			if i == j {
				continue
			}
			xj := group.Scalar().SetInt64(int64(shareJ.I))
			li.Sub(li, xj)
			denominator := group.Scalar().Sub(xi, xj)
			if denominator.Equal(group.Scalar().Zero()) {
				return nil, fmt.Errorf("duplicate x-values detected at indices %d and %d", shareI.I, shareJ.I)
			}
			li.Div(li, denominator)
		}
		li.Mul(li, shareI.V)
		secret.Add(secret, li)
	}

	return secret, nil
}
