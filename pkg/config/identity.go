package config

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/unpackdev/fdb/pkg/types"
)

// SignerKey represents the private and public keys for a specific signer type.
type SignerKey struct {
	SigningPrivateKey string `yaml:"privateKey"` // Hex-encoded private key
	SigningPublicKey  string `yaml:"publicKey"`  // Hex-encoded public key
}

// ThresholdSignerKey represents the serialized form of a Threshold BLS signer.
type ThresholdSignerKey struct {
	Shares      map[int]string `yaml:"shares"`
	Threshold   int            `yaml:"threshold"`
	Commitments string         `yaml:"commitments"` // Add this field
}

// KeyRoles holds the RBAC roles and permissions for an Account.
type KeyRoles struct {
	Roles            []types.Role                      `yaml:"roles"`
	ExtraPermissions map[types.Role][]types.Permission `yaml:"extraPermissions"`
}

// Key represents a single identity key configuration with multiple signers.
type Key struct {
	Name            string                         `yaml:"name"`    // Name of the identity key
	Address         types.Address                  `yaml:"address"` // Hex-encoded address based on the signer that is used
	PeerID          peer.ID                        `yaml:"peerID"`  // Associated peer.ID
	PeerPrivateKey  string                         `yaml:"peerPrivateKey"`
	PeerPublicKey   string                         `yaml:"peerPublicKey"`
	Signers         map[types.SignerType]SignerKey `yaml:"signers"`          // Map of signer types to their keys
	Comment         string                         `yaml:"comment"`          // Optional comment
	ThresholdSigner *ThresholdSignerKey            `yaml:"thresholdSigners"` // Map identifier to ThresholdSignerKey
	RBAC            *KeyRoles                      `yaml:"rbac"`
}

// Identity defines the configuration for identity management.
type Identity struct {
	Enabled  bool   `yaml:"enabled"`  // Enable or disable identity management
	BasePath string `yaml:"basePath"` // Base path for storing identity files
	Keys     []Key  `yaml:"keys"`     // List of keys used in identity management
}
