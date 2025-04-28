package accounts

import (
	"context"
	"fmt"
	"testing"

	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerd/pkg/rbac"
	"github.com/peerdns/peerd/pkg/signatures"
	"github.com/peerdns/peerd/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccountSign tests the Sign method of the Account.
func TestAccountSign(t *testing.T) {
	// Set up test environment
	cfg, log := setupTestEnvironment(t)

	// Initialize RBAC Manager with default roles
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rbacMgr, rmErr := rbac.NewManager(
		ctx,
		rbac.WithDefaultRoles(),
	)
	require.NoError(t, rmErr, "Failed to initialize RBAC Manager")

	// Initialize the Store with RBAC Manager
	_, err := NewStore(cfg, log, rbacMgr)
	require.NoError(t, err, "Failed to create store")

	// Initialize Signers map
	signers := make(map[signatures.SignerType]signatures.Signer)
	edSigner, err := signatures.NewEd25519Signer()
	require.NoError(t, err, "Failed to create Ed25519 signer")
	signers[signatures.Ed25519SignerType] = edSigner

	// Generate accounts with and without signing permission
	peerPrivKey, peerPubKey, err := libp2pCrypto.GenerateKeyPair(libp2pCrypto.Ed25519, 2048)
	require.NoError(t, err, "Failed to generate key pair")
	peerID, err := libp2pPeer.IDFromPublicKey(peerPubKey)
	require.NoError(t, err, "Failed to derive peer ID")

	accountWithPermission, err := NewAccount(
		peerID,
		peerPrivKey,
		peerPubKey,
		signers,
		nil,
		"Account With Permission",
		"Testing sign functionality.",
		[]types.Role{rbac.RoleAdmin}, // RoleAdmin has PermissionSignTransactions
		nil,
		rbacMgr,
	)
	require.NoError(t, err)
	require.NotNil(t, accountWithPermission)

	// Account without signing permission
	peerPrivKey2, peerPubKey2, err := libp2pCrypto.GenerateKeyPair(libp2pCrypto.Ed25519, 2048)
	require.NoError(t, err)
	peerID2, err := libp2pPeer.IDFromPublicKey(peerPubKey2)
	require.NoError(t, err)

	accountWithoutPermission, err := NewAccount(
		peerID2,
		peerPrivKey2,
		peerPubKey2,
		signers,
		nil,
		"Account Without Permission",
		"Testing sign functionality without permission.",
		[]types.Role{rbac.RoleUser}, // RoleUser does not have PermissionSignTransactions
		nil,
		rbacMgr,
	)
	require.NoError(t, err)
	require.NotNil(t, accountWithoutPermission)

	data := []byte("Hello, PeerDNS!")
	signerType := signatures.Ed25519SignerType

	tests := []struct {
		name        string
		data        []byte
		signer      *Account
		signerType  signatures.SignerType
		expectError bool
	}{
		{
			name:        "Sign with Valid Signer and Permission",
			data:        data,
			signer:      accountWithPermission,
			signerType:  signerType,
			expectError: false,
		},
		{
			name:        "Sign without Permission",
			data:        data,
			signer:      accountWithoutPermission,
			signerType:  signerType,
			expectError: true,
		},
		{
			name:        "Sign with Nil Signer",
			data:        data,
			signer:      nil,
			signerType:  signerType,
			expectError: true,
		},
		{
			name:        "Sign with Empty Data",
			data:        []byte(""),
			signer:      accountWithPermission,
			signerType:  signerType,
			expectError: true,
		},
		{
			name:        "Sign with Unsupported Signer Type",
			data:        data,
			signer:      accountWithPermission,
			signerType:  "UnsupportedSignerType",
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			var signature []byte
			var err error
			if tt.signer != nil {
				signature, err = tt.signer.Sign(tt.signerType, tt.data)
			} else {
				err = fmt.Errorf("signer is nil")
			}

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, signature)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, signature)
			}
		})
	}
}

// TestAccountVerify tests the Verify method of the Account.
func TestAccountVerify(t *testing.T) {
	// Set up test environment
	cfg, log := setupTestEnvironment(t)

	// Initialize RBAC Manager with default roles
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rbacMgr, rmErr := rbac.NewManager(
		ctx,
		rbac.WithDefaultRoles(),
	)
	require.NoError(t, rmErr, "Failed to initialize RBAC Manager")

	// Initialize the Store with RBAC Manager
	_, err := NewStore(cfg, log, rbacMgr)
	require.NoError(t, err, "Failed to create store")

	// Initialize Signers map
	signers := make(map[signatures.SignerType]signatures.Signer)
	edSigner, err := signatures.NewEd25519Signer()
	require.NoError(t, err, "Failed to create Ed25519 signer")
	signers[signatures.Ed25519SignerType] = edSigner

	// Generate accounts with and without verification permission
	peerPrivKey, peerPubKey, err := libp2pCrypto.GenerateKeyPair(libp2pCrypto.Ed25519, 2048)
	require.NoError(t, err, "Failed to generate key pair")
	peerID, err := libp2pPeer.IDFromPublicKey(peerPubKey)
	require.NoError(t, err, "Failed to derive peer ID")

	accountWithPermission, err := NewAccount(
		peerID,
		peerPrivKey,
		peerPubKey,
		signers,
		nil,
		"Account With Permission",
		"Testing verify functionality.",
		[]types.Role{rbac.RoleAdmin}, // RoleAdmin has PermissionVerifySignatures
		nil,
		rbacMgr,
	)
	require.NoError(t, err)
	require.NotNil(t, accountWithPermission)

	// Account without verification permission
	peerPrivKey2, peerPubKey2, err := libp2pCrypto.GenerateKeyPair(libp2pCrypto.Ed25519, 2048)
	require.NoError(t, err)
	peerID2, err := libp2pPeer.IDFromPublicKey(peerPubKey2)
	require.NoError(t, err)

	accountWithoutPermission, err := NewAccount(
		peerID2,
		peerPrivKey2,
		peerPubKey2,
		signers,
		nil,
		"Account Without Permission",
		"Testing verify functionality without permission.",
		[]types.Role{rbac.RoleUser}, // RoleUser does not have PermissionVerifySignatures
		nil,
		rbacMgr,
	)
	require.NoError(t, err)
	require.NotNil(t, accountWithoutPermission)

	data := []byte("Hello, PeerDNS!")
	signerType := signatures.Ed25519SignerType
	validSignature, err := accountWithPermission.Sign(signerType, data)
	require.NoError(t, err)
	require.NotNil(t, validSignature)

	// Create an invalid signature
	otherData := []byte("Other Data")
	otherSignature, err := accountWithPermission.Sign(signerType, otherData)
	require.NoError(t, err)
	require.NotNil(t, otherSignature)

	tests := []struct {
		name        string
		data        []byte
		signature   []byte
		account     *Account
		signerType  signatures.SignerType
		expectValid bool
		expectError bool
	}{
		{
			name:        "Verify Valid Signature with Permission",
			data:        data,
			signature:   validSignature,
			account:     accountWithPermission,
			signerType:  signerType,
			expectValid: true,
			expectError: false,
		},
		{
			name:        "Verify Invalid Signature with Permission",
			data:        data,
			signature:   otherSignature,
			account:     accountWithPermission,
			signerType:  signerType,
			expectValid: false,
			expectError: false,
		},
		{
			name:        "Verify with Nil Signature",
			data:        data,
			signature:   nil,
			account:     accountWithPermission,
			signerType:  signerType,
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Verify with Nil Account",
			data:        data,
			signature:   validSignature,
			account:     nil,
			signerType:  signerType,
			expectValid: false,
			expectError: true,
		},
		{
			name:        "Verify without Permission",
			data:        data,
			signature:   validSignature,
			account:     accountWithoutPermission,
			signerType:  signerType,
			expectValid: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			var valid bool
			var err error
			if tt.account != nil {
				valid, err = tt.account.Verify(tt.signerType, tt.data, tt.signature)
			} else {
				err = fmt.Errorf("account is nil")
			}

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, valid)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectValid, valid)
			}
		})
	}
}
