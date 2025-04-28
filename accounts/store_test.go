// pkg/accounts/store_test.go
package accounts

import (
	"context"
	"github.com/peerdns/peerd/pkg/config"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdns/peerd/pkg/rbac"
	"github.com/peerdns/peerd/pkg/signatures"
	"github.com/peerdns/peerd/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreConcurrency(t *testing.T) {
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
	store, err := NewStore(cfg, log, rbacMgr)
	require.NoError(t, err, "Failed to create store")

	// Initialize Signers map
	signers := make(map[signatures.SignerType]signatures.Signer)
	edSigner, err := signatures.NewEd25519Signer()
	require.NoError(t, err, "Failed to create Ed25519 signer")
	signers[signatures.Ed25519SignerType] = edSigner
	secSigner, sErr := signatures.NewSecp256k1Signer()
	require.NoError(t, sErr, "Failed to create secp256k1 signer")
	signers[signatures.Secp256k1SignerType] = secSigner

	// Generate a new peer ID and key pair
	peerPrivKey, peerPubKey, err := libp2pCrypto.GenerateKeyPair(libp2pCrypto.Ed25519, 2048)
	require.NoError(t, err, "Failed to generate key pair")
	peerID, err := libp2pPeer.IDFromPublicKey(peerPubKey)
	require.NoError(t, err, "Failed to derive peer ID")

	// Create an Account with required parameters
	account, err := NewAccount(
		peerID,
		peerPrivKey,
		peerPubKey,
		signers,
		nil, // No threshold signer for simplicity
		"Concurrency Test Account",
		"Testing Store Concurrency",
		[]types.Role{rbac.RoleAdmin}, // Assign Admin role for testing
		nil,                          // No extra permissions
		rbacMgr,
	)
	require.NoError(t, err, "Failed to create account")
	require.NotNil(t, account, "Account should not be nil")

	// Define variables to be used across test cases
	var wg sync.WaitGroup
	numOperations := 100

	// Perform concurrent save and delete operations
	for i := 0; i < numOperations; i++ {
		wg.Add(2)

		// Concurrently save the account
		go func(i int) {
			defer wg.Done()
			err := store.Save(account)
			if err != nil {
				t.Errorf("Failed to save account concurrently: %v", err)
			}
		}(i)

		// Concurrently delete the account
		go func(i int) {
			defer wg.Done()
			err := store.Delete(peerID)
			if err != nil && !strings.Contains(err.Error(), "account not found") {
				t.Errorf("Failed to delete account concurrently: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Final verification: the account should either exist or not, but the store should be in a consistent state
	retrievedAccount, err := store.GetByPeerID(peerID)
	if err != nil {
		// Account may have been deleted; ensure it cannot be retrieved
		assert.Error(t, err, "Account should have been deleted or not exist")
	} else {
		// Account exists; verify its integrity
		assert.NotNil(t, retrievedAccount, "Account should exist")
		assert.Equal(t, account.ID, retrievedAccount.ID, "Retrieved account ID should match")
	}

	// Clean up the temporary directory after tests
	err = os.RemoveAll(cfg.BasePath)
	require.NoError(t, err, "Failed to clean up temporary directory after Concurrency test")
}

func TestThresholdSignerSerialization(t *testing.T) {
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
	store, err := NewStore(cfg, log, rbacMgr)
	require.NoError(t, err, "Failed to create store")

	// Define account details
	accountName := "Test Threshold Signer Account"
	roles := []types.Role{rbac.RoleValidator}

	// Create the account
	account, err := store.Create(accountName, "Test account with Threshold BLS Signer", true, roles...)
	require.NoError(t, err, "Failed to create account with threshold BLS signer")
	require.NotNil(t, account, "Account should not be nil")

	// Verify that the account has a Threshold BLS signer
	thresholdSigner, exists := account.ThresholdSigners["default"]
	require.True(t, exists, "Threshold BLS signer should exist in account")
	require.NotNil(t, thresholdSigner, "Threshold BLS signer should not be nil")

	// Now, save the account to disk
	err = store.Save(account)
	require.NoError(t, err, "Failed to save account to disk")

	// Load the account from disk
	loadedAccount, err := store.GetByPeerID(account.PeerID)
	require.NoError(t, err, "Failed to load account from store")
	require.NotNil(t, loadedAccount, "Loaded account should not be nil")

	// Verify that the Threshold BLS signer is correctly deserialized
	loadedThresholdSigner, exists := loadedAccount.ThresholdSigners["default"]
	require.True(t, exists, "Threshold BLS signer should exist in loaded account")
	require.NotNil(t, loadedThresholdSigner, "Threshold BLS signer in loaded account should not be nil")

	// Verify that the loaded Threshold BLS signer works correctly
	data := []byte("Test data for signing")
	// Generate a partial signature
	shareIndex := 1 // Since in this test we have only one share
	partialSig, err := loadedThresholdSigner.Pair().(*signatures.ThresholdKeyPair).GeneratePartialSignature(shareIndex, data)
	require.NoError(t, err, "Failed to generate partial signature with loaded threshold signer")
	require.NotNil(t, partialSig, "Partial signature should not be nil")

	// Since we have only one partial signature, it is also the aggregated signature
	aggSig := partialSig

	// Verify the aggregated signature
	valid, err := loadedThresholdSigner.Verify(data, aggSig)
	require.NoError(t, err, "Failed to verify aggregated signature")
	require.True(t, valid, "Aggregated signature should be valid")

	// Clean up the temporary directory after tests
	err = os.RemoveAll(cfg.BasePath)
	require.NoError(t, err, "Failed to clean up temporary directory after test")
}

func TestThresholdSignerSerializationAndDiskPersistence(t *testing.T) {
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
	store, err := NewStore(cfg, log, rbacMgr)
	require.NoError(t, err, "Failed to create store")

	// Define account details
	accountName := "Test Threshold Signer Account"
	roles := []types.Role{rbac.RoleValidator}

	// Create the account
	account, err := store.Create(accountName, "Test account with Threshold BLS Signer", true, roles...)
	require.NoError(t, err, "Failed to create account with threshold BLS signer")
	require.NotNil(t, account, "Account should not be nil")

	// Verify that the account has a Threshold BLS signer
	thresholdSigner, exists := account.ThresholdSigners["default"]
	require.True(t, exists, "Threshold BLS signer should exist in account")
	require.NotNil(t, thresholdSigner, "Threshold BLS signer should not be nil")

	// Now, save the account to disk
	err = store.Save(account)
	require.NoError(t, err, "Failed to save account to disk")

	// Read the account file from disk
	accountFilePath := filepath.Join(cfg.BasePath, account.PeerID.String()+".yaml")
	accountFileData, err := ioutil.ReadFile(accountFilePath)
	require.NoError(t, err, "Failed to read account file from disk")

	// Parse the YAML file to confirm that thresholdSigners are saved
	var savedKey config.Key
	err = yaml.Unmarshal(accountFileData, &savedKey)
	require.NoError(t, err, "Failed to unmarshal account YAML data")

	// Check that thresholdSigners is present and has the expected data
	tsKey, exists := savedKey.ThresholdSigners["default"]
	require.True(t, exists, "Threshold signers should be present in saved account file")
	require.Equal(t, thresholdSigner.Threshold, tsKey.Threshold, "Threshold should match")

	// Ensure that shares are saved
	require.NotEmpty(t, tsKey.Shares, "Shares should be present in threshold signer")
	// You can print the shares if you want to inspect them
	t.Logf("Threshold signer shares saved to disk: %+v", tsKey.Shares)

	// Load the account from disk
	loadedStore, err := NewStore(cfg, log, rbacMgr)
	require.NoError(t, err, "Failed to create new store for loading")

	loadedAccount, err := loadedStore.GetByPeerID(account.PeerID)
	require.NoError(t, err, "Failed to load account from store")
	require.NotNil(t, loadedAccount, "Loaded account should not be nil")

	// Verify that the Threshold BLS signer is correctly deserialized
	loadedThresholdSigner, exists := loadedAccount.ThresholdSigners["default"]
	require.True(t, exists, "Threshold BLS signer should exist in loaded account")
	require.NotNil(t, loadedThresholdSigner, "Threshold BLS signer in loaded account should not be nil")
	require.Equal(t, thresholdSigner.Threshold, loadedThresholdSigner.Threshold, "Thresholds should match")

	// Verify that the loaded Threshold BLS signer works correctly
	data := []byte("Test data for signing")
	// Generate a partial signature
	shareIndex := 1 // Since in this test we have only one share
	partialSig, err := loadedThresholdSigner.Pair().(*signatures.ThresholdKeyPair).GeneratePartialSignature(shareIndex, data)
	require.NoError(t, err, "Failed to generate partial signature with loaded threshold signer")
	require.NotNil(t, partialSig, "Partial signature should not be nil")

	// Since we have only one partial signature, it is also the aggregated signature
	aggSig := partialSig

	// Verify the aggregated signature
	valid, err := loadedThresholdSigner.Verify(data, aggSig)
	require.NoError(t, err, "Failed to verify aggregated signature")
	require.True(t, valid, "Aggregated signature should be valid")

	// Clean up the temporary directory after tests
	err = os.RemoveAll(cfg.BasePath)
	require.NoError(t, err, "Failed to clean up temporary directory after test")
}
