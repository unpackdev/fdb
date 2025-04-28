// pkg/share/participant.go

package share

import (
	"context"
	dkg "go.dedis.ch/kyber/v4/share/dkg/pedersen"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/share"
)

// Participant defines the interface for a Distributed Key Generation (DKG) participant.
// It encapsulates the functionalities required for participating in the DKG protocol,
// including generating secret polynomials, computing secret shares, and signaling readiness.
type Participant interface {
	// Index returns the participant's unique index within the DKG protocol.
	// This index is typically used to identify the participant in the protocol operations.
	Index() uint32

	LagrangeIndex() uint32

	// Epoch returns the current epoch number.
	// Epochs are used to manage different rounds or periods within the DKG process.
	Epoch() Epoch

	// ID returns the participant's peer ID.
	// This ID is used for network communications and identifying the participant in the network.
	ID() peer.ID

	// DistributedPriShare returns the participant's secret share as a Kyber scalar.
	// Secret shares are used in the reconstruction of the master secret.
	DistributedPriShare() *share.PriShare

	// Commitments returns the participant's public commitments polynomial.
	// Commitments are used to verify the integrity of secret shares without revealing the secret.
	Commitments() []kyber.Point

	// Committee returns a slice of peer IDs representing the committee members.
	// The committee is responsible for coordinating the DKG protocol.
	Committee() []peer.ID

	// DistributedPublicKey returns the group's aggregated public key.
	// This key is derived from the individual commitments of all participants.
	DistributedPublicKey() kyber.Point

	// Initialize sets up the participant for a given epoch.
	// It involves retrieving the participant's index, setting the epoch, and generating the secret polynomial and commitments.
	Initialize(epoch Epoch, suite dkg.Suite, account Account) error

	Account() Account

	SetDistributedKeyShare(key *dkg.DistKeyShare) error

	// WaitForReady blocks until the participant signals readiness or the provided context times out or is canceled.
	// This is useful for synchronization purposes within the DKG protocol.
	WaitForReady(ctx context.Context, timeout time.Duration) error

	PublicKey() kyber.Point

	// Sign signs the provided data using the participant's secret share key.
	// It returns the serialized signature bytes.
	Sign(data []byte) ([]byte, error)

	// Verify verifies the signature of the provided data using the group public key.
	// It returns true if the signature is valid.
	Verify(data []byte, signature []byte) (bool, error)
}
