// pkg/signatures/threshold_bls.go

package signatures

import (
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerd/pkg/logger"
	"github.com/peerdns/peerd/pkg/share"
	"github.com/peerdns/peerd/pkg/types"
	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
	"go.dedis.ch/kyber/v4/pairing"
	kshare "go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/sign/tbls"
	"go.uber.org/zap"
)

// ThresholdBLSSigner represents a BLS signer in a threshold setting.
type ThresholdBLSSigner struct {
	logger           logger.Logger
	SignerID         peer.ID
	thresholdKeyPair *ThresholdKeyPair
	Threshold        int
	mu               deadlock.Mutex
}

func NewThresholdBLSSigner(logger logger.Logger, signerID peer.ID, participant share.Participant, threshold int) (*ThresholdBLSSigner, error) {
	n := len(participant.Commitments()) // Total number of shares

	if n == 0 {
		return nil, fmt.Errorf("number of commitments is zero")
	}

	logger.Info(
		"Constructing new threshold bls signer",
		zap.String("validator_id", participant.ID().String()),
		zap.Int("commitments_count", n),
		zap.Int("threshold", threshold),
	)

	suite := participant.Account().DkgSuite()
	return &ThresholdBLSSigner{
		logger:   logger,
		SignerID: signerID,
		thresholdKeyPair: &ThresholdKeyPair{
			logger:      logger,
			suite:       suite,
			participant: participant,
			commitments: participant.Commitments(),
			scheme:      tbls.NewThresholdSchemeOnG1(suite.(pairing.Suite)),
			poly:        kshare.NewPubPoly(suite, participant.DistributedPublicKey(), participant.Commitments()),
			n:           n,         // Set total number of shares
			t:           threshold, // Set threshold
		},
		Threshold: threshold,
	}, nil
}

func (ts *ThresholdBLSSigner) GeneratePartialSignature(data []byte) ([]byte, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.thresholdKeyPair.participant.DistributedPriShare() == nil {
		return nil, fmt.Errorf("secret share is not initialized")
	}

	return ts.thresholdKeyPair.GeneratePartialSignature(data)
}

func (ts *ThresholdBLSSigner) Address() (types.Address, error) {
	return types.ZeroAddress, errors.New("threshold bls signer account address is not yet implemented")
}

// Sign is not used directly in threshold BLS signers.
func (ts *ThresholdBLSSigner) Sign(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("direct signing is not supported in threshold BLS")
}

// VerifyAggregatedSignature verifies the aggregated signature.
func (ts *ThresholdBLSSigner) VerifyAggregatedSignature(data []byte, signature []byte) (bool, error) {
	return ts.thresholdKeyPair.VerifyAggregatedSignature(data, signature)
}

// Verify verifies the single signature.
func (ts *ThresholdBLSSigner) Verify(data []byte, signature []byte) (bool, error) {
	return ts.thresholdKeyPair.VerifyAggregatedSignature(data, signature)
}

// Pair returns the underlying key pair.
func (ts *ThresholdBLSSigner) Pair() share.KeyPair {
	return ts.thresholdKeyPair
}

// Type returns the signer type.
func (ts *ThresholdBLSSigner) Type() types.SignerType {
	return types.ThresholdBLSSignerType
}

// AggregateAndVerifySignature aggregates partial signatures and verifies the aggregated signature.
func (ts *ThresholdBLSSigner) AggregateAndVerifySignature(blockHash types.Hash, data []byte, sigs [][]byte) ([]byte, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	aggSig, err := ts.thresholdKeyPair.AggregatePartialSignatures(data, sigs)
	if err != nil {
		return nil, err
	}

	valid, err := ts.Verify(data, aggSig)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("aggregated signature is invalid")
	}

	return aggSig, nil
}

// AggregatePartialSignatures aggregates partial signatures and verifies the aggregated signature.
func (ts *ThresholdBLSSigner) AggregatePartialSignatures(blockHash types.Hash, data []byte, partialSigs [][]byte) ([]byte, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	aggSig, err := ts.thresholdKeyPair.AggregatePartialSignatures(data, partialSigs)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate partial signatures: %w", err)
	}

	valid, err := ts.Verify(data, aggSig)
	if err != nil {
		return nil, fmt.Errorf("failed to verify aggregated signature: %w", err)
	}
	if !valid {
		return nil, fmt.Errorf("aggregated signature is invalid")
	}

	return aggSig, nil
}
