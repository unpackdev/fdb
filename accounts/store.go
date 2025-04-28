// pkg/accounts/store.go

package accounts

import (
	"encoding/hex"
	"fmt"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/rbac"
	"github.com/unpackdev/fdb/share"
	"github.com/unpackdev/fdb/signatures"
	"github.com/unpackdev/fdb/types"

	"github.com/sasha-s/go-deadlock"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// Store manages the persistence of Accounts using YAML files.
type Store struct {
	cfg       config.Identity
	rbacMgr   *rbac.Manager
	keys      map[libp2pPeer.ID]*Account
	addresses map[types.Address]*Account
	logger    logger.Logger
	mu        deadlock.RWMutex
}

// NewStore initializes a new Store with the identity configuration.
func NewStore(cfg config.Identity, logger logger.Logger, rbacMgr *rbac.Manager) (*Store, error) {
	if err := signatures.InitBLS(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize BLS library")
	}

	store := &Store{
		cfg:       cfg,
		logger:    logger,
		rbacMgr:   rbacMgr,
		keys:      make(map[libp2pPeer.ID]*Account),
		addresses: make(map[types.Address]*Account),
	}

	// Load existing accounts from disk
	if err := store.Load(); err != nil {
		return nil, errors.Wrap(err, "failed to load accounts from store")
	}

	return store, nil
}

// Load reads all Accounts from individual YAML files and reconstructs them in the Store's keys map.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// List all YAML files in the base path directory
	files, err := filepath.Glob(filepath.Join(s.cfg.BasePath, "*.yaml"))
	if err != nil {
		return errors.Wrap(err, "failed to list account files")
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %s", file)
		}

		// Unmarshal the file content into config.Key struct
		var key config.Key
		if err := yaml.Unmarshal(data, &key); err != nil {
			return errors.Wrapf(err, "failed to unmarshal account from file %s", file)
		}

		// Decode the peer ID from the key
		peerID, err := libp2pPeer.Decode(key.PeerID.String())
		if err != nil {
			return errors.Wrapf(err, "failed to decode peer ID from file %s", file)
		}

		// Decode and reconstruct the private key
		privKeyBytes, err := hex.DecodeString(key.PeerPrivateKey)
		if err != nil {
			return errors.Wrapf(err, "failed to decode private key from file %s", file)
		}
		privKey, err := libp2pCrypto.UnmarshalPrivateKey(privKeyBytes)
		if err != nil {
			return errors.Wrapf(err, "failed to unmarshal private key from file %s", file)
		}

		// Decode and reconstruct the public key
		pubKeyBytes, err := hex.DecodeString(key.PeerPublicKey)
		if err != nil {
			return errors.Wrapf(err, "failed to decode public key from file %s", file)
		}
		pubKey, err := libp2pCrypto.UnmarshalPublicKey(pubKeyBytes)
		if err != nil {
			return errors.Wrapf(err, "failed to unmarshal public key from file %s", file)
		}

		// Reconstruct the signers
		signers := make(map[types.SignerType]share.Signer)

		for signerType, signerKey := range key.Signers {
			privateKeyBytes, err := hex.DecodeString(signerKey.SigningPrivateKey)
			if err != nil {
				return errors.Wrapf(err, "failed to decode signing private key for signer type %s from file %s", signerType, file)
			}
			publicKeyBytes, err := hex.DecodeString(signerKey.SigningPublicKey)
			if err != nil {
				return errors.Wrapf(err, "failed to decode signing public key for signer type %s from file %s", signerType, file)
			}

			var signer share.Signer
			switch signerType {
			case types.BlsSignerType:
				signer, err = signatures.NewBLSSignerWithKeys(privateKeyBytes, publicKeyBytes)
			case types.Ed25519SignerType:
				signer, err = signatures.NewEd25519SignerWithKeys(privateKeyBytes, publicKeyBytes)
				if err != nil {
					return errors.Wrapf(err, "failed to initialize Ed25519 signer from file %s", file)
				}
			case types.SchnorrSignerType:
				signer, err = signatures.NewSchnorrSignerWithKeys(privateKeyBytes, publicKeyBytes)
			case types.Secp256k1SignerType:
				signer, err = signatures.NewSecp256k1SignerWithKeys(privateKeyBytes, publicKeyBytes)
			default:
				return fmt.Errorf("unsupported signer type: %s", signerType)
			}
			if err != nil {
				return errors.Wrapf(err, "failed to initialize %s signer from file %s", signerType, file)
			}
			signers[signerType] = signer
		}

		// Reconstruct ThresholdBLSSigner if present
		/*		var thresholdSigner *signatures.ThresholdBLSSigner
				suite := bn256.NewSuite()
				if key.ThresholdSigner != nil {
					tsKey := key.ThresholdSigner
					priShares := make([]*kshare.PriShare, 0, len(tsKey.Shares))
					var secShare *kshare.PriShare // This will be our secret key
					for idx, shareHex := range tsKey.Shares {
						shareBytes, err := hex.DecodeString(shareHex)
						if err != nil {
							return errors.Wrapf(err, "failed to decode share for Threshold BLS signer from file %s", file)
						}
						priShare, err := signatures.DeserializePriShare(suite, shareBytes)
						if err != nil {
							return errors.Wrapf(err, "failed to deserialize PriShare from bytes")
						}
						priShare.I = idx
						priShares = append(priShares, priShare)

						// Assuming we only hold one share (our own), set it as the secret key
						// TODO: multiple shares, need logic to identify the correct share
						secShare = priShare
					}

					if secShare == nil {
						return errors.New("secret share could not be decoded while loading threshold signer account")
					}

					commitmentsBytes, err := hex.DecodeString(tsKey.Commitments)
					if err != nil {
						return errors.Wrap(err, "failed to decode commitments")
					}

					commitments, err := signatures.UnmarshalPubPoly(suite, commitmentsBytes)
					if err != nil {
						return errors.Wrap(err, "failed to deserialize commitments")
					}

					// Reconstruct the MasterPublicKey from the commitments
					masterPublicKey := commitments.Eval(0).V

					tkp := &signatures.ThresholdKeyPair{
						MasterPublicKey:  masterPublicKey,
						MasterPrivateKey: secShare.V,
						Shares:           priShares,
						Commitments:      commitments,
						N:                len(priShares),
						T:                tsKey.Threshold,
					}
					thresholdSigner, err = signatures.NewThresholdBLSSigner(peerID.String(), nil, tkp.T)
				}
				_ = thresholdSigner*/

		// Reconstruct Roles and ExtraPermissions
		roles := make([]types.Role, 0)
		if key.RBAC != nil {
			for _, role := range key.RBAC.Roles {
				roles = append(roles, role)
				permissions, pErr := s.rbacMgr.GetPermissionsForRole(role)
				if pErr != nil {
					return errors.Wrapf(err, "failed to get permissions for role %s", role)
				}

				if err := s.rbacMgr.AssignRole(role, permissions...); err != nil {
					return errors.Wrapf(err, "failed to assign role %s from file %s", role, file)
				}
			}

			// Assign ExtraPermissions if any
			if key.RBAC.ExtraPermissions != nil {
				for epRole, ePermissions := range key.RBAC.ExtraPermissions {
					if err := s.rbacMgr.AssignRole(types.Role(epRole), ePermissions...); err != nil {
						return errors.Wrapf(err, "failed to assign extra permission %v to role %s from file %s", ePermissions, epRole, file)
					}
				}
			}
		}

		// Construct the Account object from the loaded data
		account, err := NewAccount(s.logger, peerID, privKey, pubKey, types.Ed25519SignerType, signers, nil, key.Name, key.Comment, roles, key.RBAC.ExtraPermissions, s.rbacMgr)
		if err != nil {
			return errors.Wrap(err, "failed to initialize account")
		}

		// Store the Account in the keys and addresses maps
		s.keys[peerID] = account
		s.addresses[account.address] = account
	}

	return nil
}

// Create generates a new Account and stores it as a YAML file.
func (s *Store) Create(name, comment string, signer types.SignerType, persist bool, roles ...types.Role) (*Account, error) {
	s.mu.RLock()
	// Check if an Account already exists with this name in memory
	for _, account := range s.keys {
		if account.name == name {
			s.mu.RUnlock()
			return nil, fmt.Errorf("account with name %s already exists", name)
		}
	}
	s.mu.RUnlock()

	// Choose the key type based on the signerType
	var peerPrivKey libp2pCrypto.PrivKey
	var peerPubKey libp2pCrypto.PubKey
	var err error

	switch signer {
	case types.Secp256k1SignerType:
		// Generate Secp256k1 keypair for peer.ID
		peerPrivKey, peerPubKey, err = libp2pCrypto.GenerateKeyPair(libp2pCrypto.Secp256k1, -1)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate Secp256k1 key pair for peer ID")
		}
	case types.Ed25519SignerType:
		// Generate Ed25519 keypair for peer.ID
		peerPrivKey, peerPubKey, err = libp2pCrypto.GenerateKeyPair(libp2pCrypto.Ed25519, -1)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate Ed25519 key pair for peer ID")
		}
	default:
		// Default to Ed25519
		peerPrivKey, peerPubKey, err = libp2pCrypto.GenerateKeyPair(libp2pCrypto.Ed25519, -1)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate Ed25519 key pair for peer ID")
		}
	}

	// Derive the peer.ID from the Secp256k1 public key
	peerID, err := libp2pPeer.IDFromPublicKey(peerPubKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to derive peer ID")
	}

	// Initialize Signers map
	signers := make(map[types.SignerType]share.Signer)

	// Generate BLS signer
	blsSigner, err := signatures.NewBLSSigner()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create BLS signer")
	}
	signers[types.BlsSignerType] = blsSigner

	// Generate Ed25519 signer
	ed25519Signer, err := signatures.NewEd25519Signer()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Ed25519 signer")
	}
	signers[types.Ed25519SignerType] = ed25519Signer

	// Generate Schnorr signer
	schnorrSigner, err := signatures.NewSchnorrSigner()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Schnorr signer")
	}
	signers[types.SchnorrSignerType] = schnorrSigner

	// Generate Secp256k1 signer
	secp256k1Signer, err := signatures.NewSecp256k1Signer()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Secp256k1 signer")
	}
	signers[types.Secp256k1SignerType] = secp256k1Signer

	/*	// Create a new ThresholdBLSSigner
		tkp := &signatures.ThresholdKeyPair{}
		nValidators := 1 // Set to the actual number of validators
		threshold := 1   // Set to the desired threshold (must be ≤ nValidators)
		if err := tkp.GenerateThresholdKey(nValidators, threshold); err != nil {
			return nil, errors.Wrap(err, "failed to generate Threshold BLS key pair")
		}

		thresholdSigner, err := signatures.NewThresholdBLSSigner(peerID.String(), nil, threshold)*/

	// Create a new Account with the derived peer ID and keys
	account, aErr := NewAccount(s.logger, peerID, peerPrivKey, peerPubKey, types.Ed25519SignerType, signers, nil, name, comment, roles, nil, s.rbacMgr)
	if aErr != nil {
		return nil, errors.Wrap(aErr, "failed to create account")
	}

	// Assign roles to the account using RBAC Manager
	if len(roles) == 0 {
		// Assign a default role if no roles are provided
		defaultRole := rbac.RoleUser
		rPermissions, rpErr := s.rbacMgr.GetPermissionsForRole(defaultRole)
		if rpErr != nil {
			return nil, errors.Wrap(rpErr, "failed to get permissions for default role")
		}

		err = account.AssignRole(defaultRole, rPermissions...)
		if err != nil {
			return nil, errors.Wrap(err, "failed to assign default role to account")
		}
	} else {
		// Assign provided roles
		for _, role := range roles {
			rPermissions, rpErr := s.rbacMgr.GetPermissionsForRole(role)
			if rpErr != nil {
				return nil, errors.Wrap(rpErr, "failed to get permissions for default role")
			}

			err = account.AssignRole(role, rPermissions...)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to assign role %s to account", role)
			}
		}
	}

	// Save the Account to a YAML file if persistence is enabled
	if persist {
		if err := s.Save(account); err != nil {
			return nil, errors.Wrap(err, "failed to store Account")
		}
	} else {
		s.mu.Lock()
		s.keys[peerID] = account
		s.addresses[account.address] = account
		s.mu.Unlock()
	}

	return account, nil
}

// Save persists an Account to a YAML file in the configured base path.
func (s *Store) Save(account *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure the base directory exists
	if err := os.MkdirAll(s.cfg.BasePath, 0755); err != nil {
		return errors.Wrap(err, "failed to create base directory for accounts")
	}

	// Marshal the private key into a byte slice
	privKeyBytes, err := libp2pCrypto.MarshalPrivateKey(account.masterPrivateKey)
	if err != nil {
		return errors.Wrap(err, "failed to marshal master private key")
	}

	// Marshal the public key into a byte slice
	pubKeyBytes, err := libp2pCrypto.MarshalPublicKey(account.masterPublicKey)
	if err != nil {
		return errors.Wrap(err, "failed to marshal master public key")
	}

	// Serialize the signers
	signers := make(map[types.SignerType]config.SignerKey)

	for signerType, signer := range account.signers {
		privateKeyBytes, err := signer.Pair().SerializePrivate()
		if err != nil {
			return errors.Wrapf(err, "failed to serialize private key for signer type %s", signerType)
		}
		publicKeyBytes, err := signer.Pair().SerializePublic()
		if err != nil {
			return errors.Wrapf(err, "failed to serialize public key for signer type %s", signerType)
		}

		signerKey := config.SignerKey{
			SigningPrivateKey: hex.EncodeToString(privateKeyBytes),
			SigningPublicKey:  hex.EncodeToString(publicKeyBytes),
		}
		signers[signerType] = signerKey
	}

	// Serialize threshold signers
	/*	var thresholdSigner *config.ThresholdSignerKey
		suite := bn256.NewSuite()
		if account.thresholdSigner != nil {
			serializedShares := make(map[int]string)
			tkp := account.thresholdSigner.Pair().(*signatures.ThresholdKeyPair)
			for _, tkpShare := range tkp.Shares {
				shareBytes, sbErr := signatures.SerializePriShare(suite, tkpShare)
				if sbErr != nil {
					return errors.Wrapf(sbErr, "failed to serialize PriShare")
				}
				serializedShares[tkpShare.I] = hex.EncodeToString(shareBytes) // Use share.I as key
			}

			commitmentsBytes, err := signatures.MarshalPubPoly(suite, tkp.Commitments)
			if err != nil {
				return errors.Wrap(err, "failed to serialize commitments")
			}

			thresholdSigner = &config.ThresholdSignerKey{
				Shares:      serializedShares,
				Threshold:   account.thresholdSigner.Threshold,
				Commitments: hex.EncodeToString(commitmentsBytes),
			}
		}*/

	// Serialize roles and extra permissions
	keyRoles := &config.KeyRoles{
		Roles:            []types.Role{},
		ExtraPermissions: make(map[types.Role][]types.Permission),
	}

	// Assign roles
	for _, role := range account.roles {
		keyRoles.Roles = append(keyRoles.Roles, role)
	}

	// Assign extra permissions if any
	for role, permissions := range account.extraPermissions {
		keyRoles.ExtraPermissions[role] = permissions
	}

	// Create a config.Key object with all key data in hexadecimal format
	toWrite := config.Key{
		Name:           account.name,
		PeerID:         account.peerID,
		PeerPrivateKey: hex.EncodeToString(privKeyBytes),
		PeerPublicKey:  hex.EncodeToString(pubKeyBytes),
		Signers:        signers,
		Address:        account.Address(),
		Comment:        account.comment,
		//ThresholdSigner: thresholdSigner,
		RBAC: keyRoles, // Include RBAC roles and permissions
	}

	// Marshal the config.Key into YAML
	data, err := yaml.Marshal(toWrite)
	if err != nil {
		return errors.Wrap(err, "failed to marshal Account to YAML")
	}

	// Write the YAML data to a file named with the peer ID
	filePath := filepath.Join(s.cfg.BasePath, account.peerID.String()+".yaml")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write Account file")
	}

	s.logger.Info(
		"Successfully saved account",
		zap.String("id", account.id),
		zap.String("name", account.name),
		zap.String("address", account.address.Hex()),
		zap.String("path", filePath),
	)

	s.keys[account.peerID] = account
	s.addresses[account.address] = account

	return nil
}

// GetByPeerID returns an Account by its peer.ID if it exists in memory.
func (s *Store) GetByPeerID(peerID libp2pPeer.ID) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, exists := s.keys[peerID]
	if !exists {
		return nil, fmt.Errorf("account not found for peer ID: %s", peerID.String())
	}

	return account, nil
}

// GetByName returns an Account by its name if it exists in memory.
func (s *Store) GetByName(name string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, account := range s.keys {
		if account.name == name {
			return account, nil
		}
	}

	return nil, fmt.Errorf("account not found for name: %s", name)
}

// GetByAddress returns an Account by its Address if it exists in memory.
func (s *Store) GetByAddress(addr types.Address) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, exists := s.addresses[addr]
	if !exists {
		return nil, fmt.Errorf("account not found for address: %s", addr.Hex())
	}

	return account, nil
}

// GetByRole returns an Account by its role if it exists in memory.
func (s *Store) GetByRole(role types.Role) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, account := range s.keys {
		for _, aRole := range account.roles {
			if aRole == role {
				return account, nil
			}
		}
	}

	return nil, fmt.Errorf("account not found for role: %s", role)
}

// List returns a list of all Accounts stored in memory.
func (s *Store) List() ([]*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var accounts []*Account
	for _, account := range s.keys {
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// Delete removes an Account from the storage by deleting its corresponding YAML file and removing it from memory.
func (s *Store) Delete(peerID libp2pPeer.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if the Account exists in memory
	account, exists := s.keys[peerID]
	if !exists {
		return fmt.Errorf("account not found for peer ID: %s", peerID.String())
	}

	// Remove from memory
	delete(s.keys, peerID)
	delete(s.addresses, account.address)

	// Remove the file from the disk
	filePath := filepath.Join(s.cfg.BasePath, peerID.String()+".yaml")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to delete Account file")
	}

	// Log successful deletion
	s.logger.Info(
		"Successfully deleted account",
		zap.String("id", account.id),
		zap.String("name", account.name),
		zap.String("address", account.address.Hex()),
		zap.String("path", filePath),
	)

	return nil
}
