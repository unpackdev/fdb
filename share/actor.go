// pkg/share/actor.go

package share

import (
	"crypto"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/logger"
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

// GetPublicKeyBytes returns the public key bytes of the Actor.
func (v *Actor) GetPublicKeyBytes() ([]byte, error) {
	return libp2pCrypto.MarshalPublicKey(v.account.MasterPublicKey())
}
