// pkg/share/account.go

package share

import (
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/pkg/types"
)

// Account defines the interface for a Decentralized Identifier (DID) with associated cryptographic keys and permissions.
// It encapsulates functionalities required for managing cryptographic operations, roles, and permissions.
type Account interface {
	// ID returns the unique identifier of the account, typically the string representation of the peer ID.
	ID() string

	// Address returns the derived address from the MasterPublicKey.
	Address() types.Address

	// PeerID returns the associated libp2p PeerID.
	PeerID() peer.ID

	// Name returns the name of the account.
	Name() string

	// Comment returns the optional comment or description of the account.
	Comment() string

	// MasterPrivateKey returns the master private key associated with the account.
	MasterPrivateKey() libp2pCrypto.PrivKey

	// MasterPublicKey returns the master public key associated with the account.
	MasterPublicKey() libp2pCrypto.PubKey

	// Roles returns the list of roles assigned to the account.
	Roles() []types.Role

	// ExtraPermissions returns the additional permissions associated with each role.
	ExtraPermissions() map[types.Role][]types.Permission

	// AssignRole assigns a new role with specified permissions to the account.
	AssignRole(role types.Role, permissions ...types.Permission) error

	// RemoveRole removes an existing role from the account.
	RemoveRole(role types.Role) error

	// HasPermission checks if the account has the specified permission.
	HasPermission(permission types.Permission) bool

	// MarshalPublicKey marshals the account's master public key into bytes.
	MarshalPublicKey() ([]byte, error)
}
