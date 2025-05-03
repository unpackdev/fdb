// pkg/accounts/account.go

package accounts

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/share"
	"github.com/unpackdev/fdb/pkg/types"

	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
)

// Ensure Account implements the share.Account interface
var _ share.Account = (*Account)(nil)

// Account represents a Decentralized Identifier with associated cryptographic keys and permissions.
type Account struct {
	logger           logger.Logger
	rbacMgr          *rbac.Manager
	id               string               // String representation of the peer ID
	signerType       types.SignerType     // SignerType associated with the account
	address          types.Address        // Derived address from MasterPublicKey
	peerID           peer.ID              // Associated peer.ID object for easier manipulation
	name             string               // Name of the account for descriptive purposes
	comment          string               // Optional comment or description for the account
	masterPrivateKey libp2pCrypto.PrivKey // Private key associated with libp2p PeerID, used as the master private key for all signers
	masterPublicKey  libp2pCrypto.PubKey  // Public key associated with libp2p PeerID, used as the master public key for all signers
	roles            []types.Role
	extraPermissions map[types.Role][]types.Permission
	mu               deadlock.RWMutex // Mutex for thread-safe operations
}

// NewConsensusAccount initializes Account used by the validators.
// TODO: This needs to be extended with roles and many more things but for now it's this
func NewConsensusAccount(logger logger.Logger, peerId peer.ID, address types.Address, pubKey libp2pCrypto.PubKey, roles []types.Role) (*Account, error) {
	account := &Account{
		logger:          logger,
		peerID:          peerId,
		id:              peerId.String(),
		address:         address,
		masterPublicKey: pubKey,
		roles:           roles,
	}

	return account, nil
}

// NewAccount initializes a new Account with all the cryptographic keys and metadata.
func NewAccount(
	logger logger.Logger,
	peerID peer.ID,
	peerSk libp2pCrypto.PrivKey,
	peerPk libp2pCrypto.PubKey,
	signerType types.SignerType,
	name, comment string,
	roles []types.Role,
	extraPermissions map[types.Role][]types.Permission,
	rbacMgr *rbac.Manager,
) (*Account, error) {

	// Null and validity checks
	if peerID == "" {
		return nil, errors.New("peerID cannot be empty")
	}
	if peerSk == nil {
		return nil, errors.New("peerSk (MasterPrivateKey) cannot be nil")
	}
	if peerPk == nil {
		return nil, errors.New("peerPk (MasterPublicKey) cannot be nil")
	}

	// Marshal the public key bytes
	pubKeyBytes, err := libp2pCrypto.MarshalPublicKey(peerPk)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal MasterPublicKey")
	}

	// Compute account ID using SignerType
	accountID := ComputeAccountID(signerType, pubKeyBytes)

	// Compute address from the master public key bytes
	address, err := computeAddressFromPublicKey(pubKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to derive address from MasterPublicKey")
	}

	account := &Account{
		logger:           logger,
		id:               accountID,
		signerType:       signerType,
		peerID:           peerID,
		address:          address,
		name:             name,
		comment:          comment,
		masterPrivateKey: peerSk,
		masterPublicKey:  peerPk,
		roles:            roles,
		extraPermissions: extraPermissions,
		rbacMgr:          rbacMgr,
	}

	return account, nil
}

// ID returns the unique identifier of the account.
func (a *Account) ID() string {
	return a.id
}

// Address returns the derived address from the MasterPublicKey.
func (a *Account) Address() types.Address {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.address
}

// PeerID returns the associated libp2p PeerID.
func (a *Account) PeerID() peer.ID {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.peerID
}

// Name returns the name of the account.
func (a *Account) Name() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.name
}

// Comment returns the optional comment or description of the account.
func (a *Account) Comment() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.comment
}

// MasterPrivateKey returns the master private key associated with the account.
func (a *Account) MasterPrivateKey() libp2pCrypto.PrivKey {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.masterPrivateKey
}

func (a *Account) MasterKeyToECDSA() (*ecdsa.PrivateKey, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	privKeyBytes, err := a.MasterPrivateKey().Raw()
	if err != nil {
		return nil, err
	}

	return crypto.ToECDSA(privKeyBytes)
}

// MasterPublicKey returns the master public key associated with the account.
func (a *Account) MasterPublicKey() libp2pCrypto.PubKey {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.masterPublicKey
}

// ExtraPermissions returns the additional permissions associated with each role.
func (a *Account) ExtraPermissions() map[types.Role][]types.Permission {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.extraPermissions
}

func (a *Account) MarshalPublicKey() ([]byte, error) {
	return a.masterPublicKey.Raw()
}

// Sign signs the provided data using the account's master private key
func (a *Account) Sign(data []byte) ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.masterPrivateKey == nil {
		return nil, errors.New("no master private key available for signing")
	}

	// Use the libp2p private key to sign the data
	return a.masterPrivateKey.Sign(data)
}

// Verify checks if the signature is valid for the given data using the account's master public key
func (a *Account) Verify(data []byte, signature []byte) (bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.masterPublicKey == nil {
		return false, errors.New("no master public key available for verification")
	}

	// Use the libp2p public key to verify the signature
	return a.masterPublicKey.Verify(data, signature)
}
