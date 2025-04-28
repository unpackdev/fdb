// pkg/accounts/account.go

package accounts

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerd/pkg/logger"
	"github.com/peerdns/peerd/pkg/rbac"
	"github.com/peerdns/peerd/pkg/share"
	"github.com/peerdns/peerd/pkg/signatures"
	"github.com/peerdns/peerd/pkg/types"
	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
	"go.dedis.ch/kyber/v4"
	dkg "go.dedis.ch/kyber/v4/share/dkg/pedersen"
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
	dkgSuite         dkg.Suite
	dkgPrivateKey    kyber.Scalar
	dkgPublicKey     kyber.Point
	signers          map[types.SignerType]share.Signer          // Authorized signers for the account
	thresholdSigners map[types.SignerType]share.ThresholdSigner // Threshold signers
	participant      share.Participant                          // If validator or sequencer, participant information will be filled out
	roles            []types.Role
	extraPermissions map[types.Role][]types.Permission
	mu               deadlock.RWMutex // Mutex for thread-safe operations
}

// NewConsensusAccount initializes Account used by the validators.
// TODO: This needs to be extended with roles and many more things but for now it's this
func NewConsensusAccount(logger logger.Logger, peerId peer.ID, address types.Address, pubKey libp2pCrypto.PubKey, dkgPublicKey kyber.Point, roles []types.Role) (*Account, error) {
	account := &Account{
		logger:           logger,
		peerID:           peerId,
		id:               peerId.String(),
		address:          address,
		masterPublicKey:  pubKey,
		dkgPublicKey:     dkgPublicKey,
		roles:            roles,
		signers:          make(map[types.SignerType]share.Signer),
		thresholdSigners: make(map[types.SignerType]share.ThresholdSigner),
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
	signers map[types.SignerType]share.Signer,
	thresholdSigners map[types.SignerType]share.ThresholdSigner,
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
	if signers == nil || len(signers) == 0 {
		return nil, errors.New("signers map cannot be nil or empty")
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

	// Initialize the cryptographic suite
	suite, sErr := signatures.NewSuiteFactory(signatures.BLS12381)
	if sErr != nil {
		return nil, errors.Wrap(sErr, "failed to create cryptographic suite factory")
	}

	dkgPrivate := suite.Scalar().Pick(suite.RandomStream())
	dkgPublic := suite.Point().Mul(dkgPrivate, nil)

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
		dkgSuite:         suite,
		dkgPrivateKey:    dkgPrivate,
		dkgPublicKey:     dkgPublic,
		signers:          signers,
		roles:            roles,
		extraPermissions: extraPermissions,
		rbacMgr:          rbacMgr,
		thresholdSigners: make(map[types.SignerType]share.ThresholdSigner),
	}

	if thresholdSigners != nil {
		account.thresholdSigners = thresholdSigners
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

// Signers returns the map of authorized signers for the account.
func (a *Account) Signers() map[types.SignerType]share.Signer {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.signers
}

func (a *Account) SignerAddress(signerType types.SignerType) (types.Address, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	signer, exists := a.signers[signerType]
	if !exists {
		return types.ZeroAddress, fmt.Errorf("%s signer address cannot be discovered as signer is not found", signerType)
	}

	return signer.Address()
}

func (a *Account) SignerPublicKey(signerType types.SignerType) (any, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	signer, exists := a.signers[signerType]
	if !exists {
		return nil, fmt.Errorf("%s signer address cannot be discovered as signer is not found", signerType)
	}

	return signer.Pair().GetPublic(), nil
}

func (a *Account) SignerPrivateKey(signerType types.SignerType) (any, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	signer, exists := a.signers[signerType]
	if !exists {
		return nil, fmt.Errorf("%s signer address cannot be discovered as signer is not found", signerType)
	}

	return signer.Pair().GetPrivate(), nil
}

func (a *Account) ToECDSA() (*ecdsa.PrivateKey, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	signer, exists := a.signers[types.Secp256k1SignerType]
	if !exists {
		return nil, errors.New("Secp256k1 signer not found in account")
	}

	return signer.Pair().GetPrivate().(*ecdsa.PrivateKey), nil
}

// MasterPublicKey returns the master public key associated with the account.
func (a *Account) MasterPublicKey() libp2pCrypto.PubKey {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.masterPublicKey
}

func (a *Account) DkgSuite() dkg.Suite {
	return a.dkgSuite
}

func (a *Account) DkgPrivateKey() kyber.Scalar {
	return a.dkgPrivateKey
}

func (a *Account) DkgPublicKey() kyber.Point {
	return a.dkgPublicKey
}

// ThresholdSigners returns the threshold BLS signers if initialized.
func (a *Account) ThresholdSigners() map[types.SignerType]share.ThresholdSigner {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.thresholdSigners
}

// ThresholdSignerByType returns the threshold BLS signer by type if initialized.
func (a *Account) ThresholdSignerByType(tsType types.SignerType) (share.ThresholdSigner, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if found := a.thresholdSigners[tsType]; found != nil {
		return found, nil
	}
	return nil, errors.Errorf("unknown threshold signer type %s for account: %s", tsType, a.id)
}

// ThresholdSignerExists returns if threshold signer exists or not.
func (a *Account) ThresholdSignerExists(tsType types.SignerType) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if _, found := a.thresholdSigners[tsType]; found {
		return true
	}

	return false
}

// ExtraPermissions returns the additional permissions associated with each role.
func (a *Account) ExtraPermissions() map[types.Role][]types.Permission {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.extraPermissions
}

func (a *Account) InitializeThresholdSigner(sType types.SignerType, participant share.Participant, threshold int) error {
	ts, err := signatures.NewThresholdBLSSigner(a.logger, a.peerID, participant, threshold)
	if err != nil {
		return err
	}
	a.thresholdSigners[sType] = ts
	a.participant = participant
	return nil
}

func (a *Account) UpdateThresholdSigner(participant share.Participant, threshold int) error {
	ts, err := signatures.NewThresholdBLSSigner(a.logger, a.peerID, participant, threshold)
	if err != nil {
		return err
	}
	a.thresholdSigners[types.ThresholdBLSSignerType] = ts
	a.participant = participant
	return nil
}

func (a *Account) Participant() share.Participant {
	return a.participant
}

func (a *Account) MarshalPublicKey() ([]byte, error) {
	return libp2pCrypto.MarshalPublicKey(a.masterPublicKey)
}

// GetSignerByType retrieves a signer from the account's Signers map by its SignerType.
func (a *Account) GetSignerByType(st types.SignerType) (share.Signer, bool) {
	signer, found := a.signers[st]
	return signer, found
}

func (a *Account) SupportedSigners() []types.SignerType {
	toReturn := make([]types.SignerType, 0)

	for signerType, _ := range a.signers {
		toReturn = append(toReturn, signerType)
	}

	for signerType, _ := range a.thresholdSigners {
		toReturn = append(toReturn, signerType)
	}

	return toReturn
}

// Sign signs the given data using the specified signer type.
func (a *Account) Sign(signerType types.SignerType, data []byte) ([]byte, error) {
	if a == nil {
		return nil, errors.New("account cannot be nil")
	}
	if data == nil || len(data) == 0 {
		return nil, errors.New("data to sign cannot be nil or empty")
	}

	// RBAC check: ensure the account has permission to sign transactions
	//	if err := a.Authorize(rbac.PermissionSignTransactions); err != nil {
	//		return nil, err
	//	}

	signer, exists := a.signers[signerType]
	if !exists {
		return nil, fmt.Errorf("signer of type %s not found in account", signerType)
	}
	if signer == nil {
		return nil, fmt.Errorf("signer of type %s is nil", signerType)
	}

	signature, err := signer.Sign(data)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data with signer type %s: %w", signerType, err)
	}

	return signature, nil
}

// SignTx signs the given types.Transaction using the specified signer type.
func (a *Account) SignTx(signerType types.SignerType, tx share.Transaction) (share.Transaction, error) {
	if a == nil {
		return nil, errors.New("account cannot be nil")
	}
	if tx == nil {
		return nil, errors.New("transaction to sign cannot be nil")
	}

	// RBAC check: ensure the account has permission to sign transactions
	if err := a.Authorize(rbac.PermissionSignTransactions); err != nil {
		return nil, err
	}

	// Retrieve the signer from the account.
	signer, exists := a.signers[signerType]
	if !exists {
		return nil, errors.Errorf("requested signer not found for associated account: %s", signerType)
	}

	// The master key pair is used to overall sign the transaction, including signers
	masterPrivKey := a.masterPrivateKey
	if masterPrivKey == nil {
		return nil, errors.New("master private key is nil")
	}

	masterPubKey := a.masterPublicKey
	if masterPubKey == nil {
		return nil, errors.New("master public key is nil")
	}

	if err := tx.Sign(signer, masterPrivKey, masterPubKey); err != nil {
		return nil, fmt.Errorf("failed to sign transaction with signer type %s: %w", signerType, err)
	}

	return tx, nil
}

// Verify verifies the given signature for the data using the specified signer type.
func (a *Account) Verify(signerType types.SignerType, data []byte, signature []byte) (bool, error) {
	if a == nil {
		return false, errors.New("account cannot be nil")
	}
	if data == nil || len(data) == 0 {
		return false, errors.New("data to verify cannot be nil or empty")
	}
	if signature == nil || len(signature) == 0 {
		return false, errors.New("signature cannot be nil or empty")
	}

	// RBAC check: ensure the account has permission to verify signatures
	if err := a.Authorize(rbac.PermissionVerifySignatures); err != nil {
		return false, err
	}

	signer, exists := a.signers[signerType]
	if !exists {
		return false, fmt.Errorf("signer of type %s not found in account", signerType)
	}
	if signer == nil {
		return false, fmt.Errorf("signer of type %s is nil", signerType)
	}

	valid, err := signer.Verify(data, signature)
	if err != nil {
		return false, fmt.Errorf("failed to verify signature with signer type %s: %w", signerType, err)
	}

	return valid, nil
}

// AuthorizeSigner signs the signer's public key with the Account's MasterPrivateKey.
func (a *Account) AuthorizeSigner(signer share.Signer) ([]byte, error) {
	if a == nil {
		return nil, errors.New("account cannot be nil")
	}
	if signer == nil {
		return nil, errors.New("signer cannot be nil")
	}

	// Ensure the signer is part of the Signers map
	signerType := signer.Type()
	_, exists := a.signers[signerType]
	if !exists {
		return nil, fmt.Errorf("signer of type %s is not authorized in this account", signerType)
	}

	// Retrieve the signer's public key bytes
	signerPubKey, err := signer.Pair().GetPublicKeyBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get signer's public key: %w", err)
	}

	// Sign the signer's public key using the Account's MasterPrivateKey
	signature, err := a.masterPrivateKey.Sign(signerPubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign signer public key: %w", err)
	}

	return signature, nil
}

// VerifySignerAuthorization verifies that the signer is authorized by checking the authorization signature.
func (a *Account) VerifySignerAuthorization(signer share.Signer, authorizationSig []byte) error {
	if a == nil {
		return errors.New("account cannot be nil")
	}
	if signer == nil {
		return errors.New("signer cannot be nil")
	}
	if authorizationSig == nil || len(authorizationSig) == 0 {
		return errors.New("authorization signature cannot be nil or empty")
	}

	// RBAC check: ensure the account has permission to verify signatures
	if err := a.Authorize(rbac.PermissionVerifySignatures); err != nil {
		return err
	}

	// Ensure the signer is part of the Signers map
	signerType := signer.Type()
	_, exists := a.signers[signerType]
	if !exists {
		return errors.Errorf("signer of type %s is not authorized in this account", signerType)
	}

	// Retrieve the signer's public key bytes
	signerPubKey, err := signer.Pair().GetPublicKeyBytes()
	if err != nil {
		return fmt.Errorf("failed to get signer's public key: %w", err)
	}

	// Verify the authorization signature: Account's MasterPublicKey signed the Signer's PublicKey
	valid, err := a.masterPublicKey.Verify(signerPubKey, authorizationSig)
	if err != nil {
		return fmt.Errorf("failed to verify authorization signature: %w", err)
	}
	if !valid {
		return errors.New("authorization signature is invalid")
	}

	return nil
}
