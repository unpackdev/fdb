// pkg/share/actor.go

package share

import (
	"crypto"
	"crypto/sha256"
	"fmt"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/types"

	"github.com/sasha-s/go-deadlock"
	"go.dedis.ch/kyber/v4"
	"go.uber.org/zap"
)

// Actor represents an actor in the consensus (sequencer, validator).
type Actor struct {
	account       Account
	shareIndex    int
	lagrangeIndex uint32
	mu            deadlock.RWMutex
	logger        logger.Logger
}

// NewActor creates a new Actor instance.
func NewActor(account Account, shareIndex int, logger logger.Logger) *Actor {
	return &Actor{
		account:    account,
		shareIndex: shareIndex,
		logger:     logger,
	}
}

// SetShareIndex updates the share index of the Actor.
// This is important when the ActorSet changes dynamically.
func (v *Actor) SetShareIndex(index uint32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.shareIndex = int(index)
	v.logger.Info("Updated actor share index", zap.String("peer_id", v.PeerID().String()), zap.Uint32("share_index", index))
}

// ShareIndex returns the current share index of the Actor.
func (v *Actor) ShareIndex() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.shareIndex
}

func (v *Actor) SetLagrangeIndex(index uint32) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.lagrangeIndex = index
}

func (v *Actor) LagrangeIndex() uint32 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.lagrangeIndex
}

// Account returns the associated account of the Actor.
func (v *Actor) Account() Account {
	return v.account
}

// PeerID returns the peer ID of the Actor.
func (v *Actor) PeerID() peer.ID {
	return v.account.PeerID()
}

// PublicKey returns the public key of the Actor.
func (v *Actor) PublicKey() crypto.PublicKey {
	return v.account.MasterPublicKey()
}

// PrivateKey returns the private key of the Actor.
func (v *Actor) PrivateKey() crypto.PrivateKey {
	return v.account.MasterPrivateKey()
}

func (v *Actor) Participant() Participant {
	return v.account.Participant()
}

func (v *Actor) DkgPublicKey() kyber.Point {
	return v.account.DkgPublicKey()
}

// GetPublicKeyBytes returns the public key bytes of the Actor.
func (v *Actor) GetPublicKeyBytes() ([]byte, error) {
	return libp2pCrypto.MarshalPublicKey(v.account.MasterPublicKey())
}

// Signer returns the signer of the specified type.
func (v *Actor) Signer(signerType types.SignerType) Signer {
	signer, _ := v.account.GetSignerByType(signerType)
	return signer
}

// ThresholdSigner returns the signer of the specified type.
func (v *Actor) ThresholdSigner(signerType types.SignerType) ThresholdSigner {
	signer, _ := v.account.ThresholdSignerByType(signerType)
	return signer
}

// GeneratePartialSignature generates a partial signature using the Actor's threshold BLS signer.
func (v *Actor) GeneratePartialSignature(sType types.SignerType, message []byte) ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.account.ThresholdSigners() == nil {
		return nil, fmt.Errorf("actor does not have a threshold BLS signer")
	}

	signer, sErr := v.account.ThresholdSignerByType(sType)
	if sErr != nil {
		return nil, sErr
	}

	partialSig, err := signer.GeneratePartialSignature(message)
	if err != nil {
		return nil, fmt.Errorf("failed to generate partial signature: %w", err)
	}
	return partialSig, nil
}

// Sign signs the provided data using the specified signer type.
func (v *Actor) Sign(signerType types.SignerType, data []byte) ([]byte, error) {
	return v.account.Sign(signerType, data)
}

// SignPartial signs the block data and returns a PartialSignature.
func (v *Actor) SignPartial(sType types.SignerType, blockBytes []byte) (PartialSignature, error) {
	// Hash the block bytes
	hash := sha256.Sum256(blockBytes)

	partialSigBytes, err := v.GeneratePartialSignature(sType, hash[:])
	if err != nil {
		return PartialSignature{}, fmt.Errorf("failed to generate partial signature: %w", err)
	}

	return PartialSignature{
		Signature:  partialSigBytes,
		ShareIndex: v.ShareIndex(),
	}, nil
}
