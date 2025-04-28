// pkg/signatures/threshold_bls_keypair.go

package signatures

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/share"

	"github.com/sasha-s/go-deadlock"
	"go.dedis.ch/kyber/v4"
	kshare "go.dedis.ch/kyber/v4/share"
	dkg "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"go.dedis.ch/kyber/v4/sign"
	"go.dedis.ch/kyber/v4/sign/tbls"
	"go.uber.org/zap"
)

// ThresholdKeyPair represents a T-BLS key pair with shares.
type ThresholdKeyPair struct {
	logger      logger.Logger
	participant share.Participant
	commitments []kyber.Point
	n           int // Total number of shares
	t           int // Threshold
	mutex       deadlock.RWMutex
	suite       dkg.Suite
	scheme      sign.ThresholdScheme
	poly        *kshare.PubPoly
}

// Suite returns the pairing suite used.
func (tkp *ThresholdKeyPair) Suite() dkg.Suite {
	return tkp.suite
}

func (tkp *ThresholdKeyPair) Participant() share.Participant {
	return tkp.participant
}

// GeneratePartialSignature generates a partial signature.
func (tkp *ThresholdKeyPair) GeneratePartialSignature(data []byte) ([]byte, error) {
	tkp.mutex.RLock()
	defer tkp.mutex.RUnlock()

	if tkp.scheme == nil {
		return nil, errors.New("threshold scheme is not initialized")
	}

	partialSig, err := tkp.scheme.Sign(tkp.participant.DistributedPriShare(), data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate partial signature: %w", err)
	}

	// Log the partial signature index and its hex representation
	index, _ := tbls.SigShare(partialSig).Index()
	tkp.logger.Info(
		"Generated Partial Signature",
		zap.Int("index", index),
		zap.String("partial_sig", hex.EncodeToString(partialSig)),
	)

	return partialSig, nil
}

// AggregatePartialSignatures aggregates partial signatures using the message, Lagrange coefficients, threshold, and total number of shares.
func (tkp *ThresholdKeyPair) AggregatePartialSignatures(data []byte, sigs [][]byte) ([]byte, error) {
	tkp.mutex.RLock()
	defer tkp.mutex.RUnlock()

	if tkp.scheme == nil {
		return nil, errors.New("threshold scheme is not initialized")
	}

	groupPubKey := tkp.poly.Eval(0).V
	pubKeyBytes, _ := groupPubKey.MarshalBinary()

	// Recover the aggregate signature using the correct t and n
	aggSig, err := tkp.scheme.Recover(tkp.poly, data, sigs, tkp.t, tkp.n)
	if err != nil {
		return nil, fmt.Errorf("failed to recover aggregated signature: %w", err)
	}

	tkp.logger.Info(
		"Aggregated partial signatures",
		zap.String("validator_id", tkp.participant.ID().String()),
		zap.Int("commitments_count", tkp.n),
		zap.Int("threshold", tkp.t),
		zap.Int("partial_signatures_count", len(sigs)),
		zap.String("group_pub_key", hex.EncodeToString(pubKeyBytes)),
		zap.String("sig", hex.EncodeToString(aggSig)),
	)

	return aggSig, nil
}

func (tkp *ThresholdKeyPair) VerifyPartialSignature(data []byte, partialSig []byte, index uint32) (bool, error) {
	tkp.mutex.RLock()
	defer tkp.mutex.RUnlock()

	if tkp.scheme == nil {
		return false, errors.New("threshold scheme is not initialized")
	}

	err := tkp.scheme.VerifyPartial(tkp.poly, data, partialSig)
	if err != nil {
		return false, fmt.Errorf("failed to verify partial signature: %w", err)
	}

	return true, nil
}

// VerifyAggregatedSignature verifies the aggregated signature.
func (tkp *ThresholdKeyPair) VerifyAggregatedSignature(msg []byte, aggSig []byte) (bool, error) {
	tkp.mutex.RLock()
	defer tkp.mutex.RUnlock()

	if tkp.scheme == nil {
		return false, errors.New("threshold scheme is not initialized")
	}

	// Use VerifyRecovered to verify the aggregated signature
	err := tkp.scheme.VerifyRecovered(tkp.participant.DistributedPublicKey(), msg, aggSig)
	if err != nil {
		return false, fmt.Errorf("failed to verify aggregated signature: %w", err)
	}

	return true, nil
}

// SerializePrivate serializes the master private key to bytes.
func (tkp *ThresholdKeyPair) SerializePrivate() ([]byte, error) {
	return nil, errors.New("serialization of private key is not allowed")
}

// SerializePublic serializes the master public key to bytes.
func (tkp *ThresholdKeyPair) SerializePublic() ([]byte, error) {
	if tkp.participant.DistributedPriShare() == nil {
		return nil, fmt.Errorf("participant public key is nil")
	}

	return tkp.participant.DistributedPublicKey().MarshalBinary()
}

// DeserializePrivate deserializes bytes into the master private key.
func (tkp *ThresholdKeyPair) DeserializePrivate(_ []byte) error {
	return errors.New("deserialization of private share participant is not implemented")
}

// DeserializePublic deserializes bytes into the master public key.
func (tkp *ThresholdKeyPair) DeserializePublic(data []byte) error {
	pk := tkp.suite.Point()
	if err := pk.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("failed to unmarshal public key: %w", err)
	}
	return nil
}

// GetPublic returns the master public key.
func (tkp *ThresholdKeyPair) GetPublic() any {
	return tkp.participant.DistributedPublicKey()
}

// GetPublicKeyBytes returns the serialized master public key.
func (tkp *ThresholdKeyPair) GetPublicKeyBytes() ([]byte, error) {
	return tkp.SerializePublic()
}

// GetPrivate returns the master private key.
func (tkp *ThresholdKeyPair) GetPrivate() any {
	return tkp.participant.DistributedPriShare()
}

// GenerateKey generates the threshold key with existing N and T values.
func (tkp *ThresholdKeyPair) GenerateKey() error {
	return errors.New("generation of the key should be done through the DKG process")
}
