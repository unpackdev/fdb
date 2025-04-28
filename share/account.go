// pkg/share/account.go

package share

import (
	"crypto/ecdsa"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/types"

	"go.dedis.ch/kyber/v4"
	dkg "go.dedis.ch/kyber/v4/share/dkg/pedersen"
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

	DkgSuite() dkg.Suite

	DkgPrivateKey() kyber.Scalar

	DkgPublicKey() kyber.Point

	Participant() Participant

	ToECDSA() (*ecdsa.PrivateKey, error)

	// Signers returns the map of authorized signers for the account.
	Signers() map[types.SignerType]Signer

	SignerAddress(signerType types.SignerType) (types.Address, error)

	SignerPublicKey(signerType types.SignerType) (any, error)

	SignerPrivateKey(signerType types.SignerType) (any, error)

	// InitializeThresholdSigner initializes the threshold BLS signer with the given participant and threshold.
	InitializeThresholdSigner(sType types.SignerType, participant Participant, threshold int) error

	ThresholdSigners() map[types.SignerType]ThresholdSigner

	ThresholdSignerByType(sType types.SignerType) (ThresholdSigner, error)

	ThresholdSignerExists(sType types.SignerType) bool

	// Roles returns the list of roles assigned to the account.
	Roles() []types.Role

	SupportedSigners() []types.SignerType

	SupportedTransports() []types.TransportType

	SupportedProtocols() []types.ProtocolType

	// ExtraPermissions returns the additional permissions associated with each role.
	ExtraPermissions() map[types.Role][]types.Permission

	// AssignRole assigns a new role with specified permissions to the account.
	AssignRole(role types.Role, permissions ...types.Permission) error

	// RemoveRole removes an existing role from the account.
	RemoveRole(role types.Role) error

	// HasPermission checks if the account has the specified permission.
	HasPermission(permission types.Permission) bool

	// Authorize ensures the account has the required permission.
	Authorize(permission types.Permission) error

	// GetSignerByType retrieves a signer from the account's signers map by its SignerType.
	GetSignerByType(signerType types.SignerType) (Signer, bool)

	// Sign signs the given data using the specified signer type.
	Sign(signerType types.SignerType, data []byte) ([]byte, error)

	// SignTx signs the given transaction using the specified signer type.
	SignTx(signerType types.SignerType, tx Transaction) (Transaction, error)

	// Verify verifies the given signature for the data using the specified signer type.
	Verify(signerType types.SignerType, data []byte, signature []byte) (bool, error)

	// AuthorizeSigner signs the signer's public key with the account's MasterPrivateKey.
	AuthorizeSigner(signer Signer) ([]byte, error)

	// VerifySignerAuthorization verifies that the signer is authorized by checking the authorization signature.
	VerifySignerAuthorization(signer Signer, authorizationSig []byte) error

	// MarshalPublicKey marshals the account's master public key into bytes.
	MarshalPublicKey() ([]byte, error)
}
