package cmd

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/pkg/accounts"
	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"github.com/unpackdev/fdb/pkg/rbac"
	"github.com/unpackdev/fdb/pkg/types"

	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	libp2pPeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func KeystoreCommand() *cli.Command {
	return &cli.Command{
		Name:  "keystore",
		Usage: "Keystore management utilities",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Usage: "Path where configuration can be found",
				Value: "./config.yaml",
			},
		},
		Before: func(c *cli.Context) error {
			cfg, err := config.InitializeGlobalConfig(c.String("config"))
			if err != nil {
				return errors.Wrap(err, "failed to load configuration")
			}

			// Sets the global logger.
			// I hate to pass by reference logger everywhere...
			// In case you wish to use your own zap logger you can disable logger here,
			// implement your own and set the globals on your end.
			_, zlErr := logger.InitializeGlobalLogger("keygen", cfg.Logger)
			if zlErr != nil {
				return errors.Wrap(zlErr, "failure to initialize logger")
			}

			return nil
		},
		Subcommands: []*cli.Command{
			// Generate a new identity
			{
				Name:  "generate",
				Usage: "Generate new keystore account with multiple signing keys",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    "Name of the identity",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "comment",
						Usage: "Comment or description for the identity",
					},
					&cli.IntFlag{
						Name:  "consensus-validators",
						Usage: "Comment or description for the identity",
						Value: 1,
					},
					&cli.IntFlag{
						Name:  "consensus-threshold",
						Usage: "What is the threshold",
						Value: 1,
					},
					&cli.BoolFlag{
						Name:     "save",
						Usage:    "Save the identity to disk",
						Required: false,
					},
					&cli.StringSliceFlag{
						Name:     "role",
						Usage:    "Roles to assign to the identity (e.g., --role admin,validator)",
						Required: true,
					},
				},
				Action: func(cmd *cli.Context) error {
					log := logger.G()

					// Initialize global RBAC manager with default roles
					rbacMgr, rbmErr := rbac.NewManager(
						cmd.Context,
						rbac.WithDefaultRoles(),
					)
					if rbmErr != nil {
						log.Error("Failed to initialize RBAC manager", zap.Error(rbmErr))
						return rbmErr
					}

					// Initialize the Store with RBAC Manager
					store, err := accounts.NewStore(config.G().Identity, log, rbacMgr)
					if err != nil {
						log.Error("Failed to create store", zap.Error(err))
						return err
					}

					// Retrieve flag values
					name := cmd.String("name")
					comment := cmd.String("comment")
					save := cmd.Bool("save")
					roleStrings := cmd.StringSlice("role")

					// Convert role strings to types.Role
					roles := make([]types.Role, 0)
					for _, roleStr := range roleStrings {
						role := types.Role(strings.ToLower(roleStr))
						roles = append(roles, role)
					}

					// Generate a new peer ID and key pair
					peerPrivKey, peerPubKey, err := libp2pCrypto.GenerateKeyPair(libp2pCrypto.Ed25519, 2048)
					if err != nil {
						log.Error("Failed to generate key pair", zap.Error(err))
						return err
					}
					peerID, err := libp2pPeer.IDFromPublicKey(peerPubKey)
					if err != nil {
						log.Error("Failed to derive peer ID", zap.Error(err))
						return err
					}

					// Create the account
					account, err := accounts.NewAccount(
						log,
						peerID,
						peerPrivKey,
						peerPubKey,
						types.Ed25519SignerType,
						name,
						comment,
						roles,
						nil, // No extra permissions
						rbacMgr,
					)
					if err != nil {
						log.Error("Failed to create account", zap.Error(err))
						return err
					}

					// Save the account if requested
					if save {
						err = store.Save(account)
						if err != nil {
							log.Error("Failed to save account", zap.Error(err))
							return err
						}
					}

					// Convert master public key to hex
					peerPublicKeyBytes, err := account.MasterPublicKey().Raw()
					if err != nil {
						log.Warn("Failed to retrieve master public key bytes", zap.Error(err))
						peerPublicKeyBytes = []byte{}
					}

					// Prepare to display roles
					var roleNames []string
					for _, role := range account.Roles() {
						roleNames = append(roleNames, string(role))
					}

					// Print created identity information
					fmt.Printf("\n[Identity Created]\n")
					fmt.Printf("Account ID:         %s\n", account.ID())
					fmt.Printf("Address:            %s\n", account.Address())
					fmt.Printf("Peer ID:            %s\n", account.PeerID())
					fmt.Printf("Peer Public Key:    %s\n", hex.EncodeToString(peerPublicKeyBytes))
					fmt.Printf("Roles:              %s\n", strings.Join(roleNames, ", "))
					fmt.Printf("Name:               %s\n", account.Name())
					fmt.Printf("Comment:            %s\n", account.Comment())
					fmt.Printf("Saved to Disk:      %v\n\n", save)

					if save {
						fmt.Println("Identity created successfully and persisted to disk.")
					}

					return nil
				},
			},

			// List all stored identities
			{
				Name:  "list",
				Usage: "List all stored identities",
				Action: func(cmd *cli.Context) error {
					log := logger.G()

					// Initialize global RBAC manager with default roles
					rbacMgr, rbmErr := rbac.NewManager(
						cmd.Context,
						rbac.WithDefaultRoles(),
					)
					if rbmErr != nil {
						log.Error("Failed to initialize RBAC manager", zap.Error(rbmErr))
						return rbmErr
					}

					// Initialize the Store with RBAC Manager
					store, err := accounts.NewStore(config.G().Identity, log, rbacMgr)
					if err != nil {
						log.Error("Failed to create store", zap.Error(err))
						return err
					}

					accs, err := store.List()
					if err != nil {
						log.Error("Failed to list identities", zap.Error(err))
						return err
					}

					if len(accs) == 0 {
						fmt.Println("No identities found.")
						return nil
					}

					fmt.Printf("\n[Stored Identities]\n")
					for _, account := range accs {
						// Prepare to display roles
						var roleNames []string
						for _, role := range account.Roles() {
							roleNames = append(roleNames, string(role))
						}

						fmt.Printf(
							"ID: %s, Name: %s, Peer ID: %s, Address: %s, Roles: %s\n",
							account.ID(), account.Name(), account.PeerID(), account.Address().Hex(), strings.Join(roleNames, ", "),
						)
					}

					fmt.Println()
					return nil
				},
			},

			// Retrieve specific identity details by Peer ID
			{
				Name:  "get",
				Usage: "Get identity information by Peer ID",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "id",
						Usage:    "Account ID of the identity to retrieve",
						Required: true,
					},
				},
				Action: func(cmd *cli.Context) error {
					log := logger.G()

					// Initialize global RBAC manager with default roles
					rbacMgr, rbmErr := rbac.NewManager(
						cmd.Context,
						rbac.WithDefaultRoles(),
					)
					if rbmErr != nil {
						log.Error("Failed to initialize RBAC manager", zap.Error(rbmErr))
						return rbmErr
					}

					// Initialize the Store with RBAC Manager
					store, err := accounts.NewStore(config.G().Identity, log, rbacMgr)
					if err != nil {
						log.Error("Failed to create store", zap.Error(err))
						return err
					}

					peerIDStr := cmd.String("id")
					peerID, err := libp2pPeer.Decode(peerIDStr)
					if err != nil {
						log.Error("Invalid Peer ID format", zap.Error(err))
						return err
					}

					account, err := store.GetByPeerID(peerID)
					if err != nil {
						log.Error("Failed to get identity", zap.Error(err))
						return err
					}

					// Convert master public key to hex
					peerPublicKeyBytes, err := account.MasterPublicKey().Raw()
					if err != nil {
						log.Warn("Failed to retrieve master public key bytes", zap.Error(err))
						peerPublicKeyBytes = []byte{}
					}

					// Prepare to display roles
					var roleNames []string
					for _, role := range account.Roles() {
						roleNames = append(roleNames, string(role))
					}

					// Print identity information
					fmt.Printf("\n[Identity Information]\n")
					fmt.Printf("Account ID:         %s\n", account.ID())
					fmt.Printf("Address:            %s\n", account.Address())
					fmt.Printf("Peer ID:            %s\n", account.PeerID())
					fmt.Printf("Peer Public Key:    %s\n", hex.EncodeToString(peerPublicKeyBytes))
					fmt.Printf("Roles:              %s\n", strings.Join(roleNames, ", "))
					fmt.Printf("Name:               %s\n", account.Name())
					fmt.Printf("Comment:            %s\n", account.Comment())

					fmt.Println()
					return nil
				},
			},

			// Delete an identity by Peer ID
			{
				Name:  "delete",
				Usage: "Delete identity by Peer ID",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "id",
						Usage:    "Account ID of the identity to delete",
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Force delete without confirmation",
					},
				},
				Action: func(cmd *cli.Context) error {
					log := logger.G()

					// Initialize global RBAC manager with default roles
					rbacMgr, rbmErr := rbac.NewManager(
						cmd.Context,
						rbac.WithDefaultRoles(),
					)
					if rbmErr != nil {
						log.Error("Failed to initialize RBAC manager", zap.Error(rbmErr))
						return rbmErr
					}

					// Initialize the Store with RBAC Manager
					store, err := accounts.NewStore(config.G().Identity, log, rbacMgr)
					if err != nil {
						log.Error("Failed to create store", zap.Error(err))
						return err
					}

					peerIDStr := cmd.String("id")
					peerID, err := libp2pPeer.Decode(peerIDStr)
					if err != nil {
						log.Error("Invalid Peer ID format", zap.Error(err))
						return err
					}

					force := cmd.Bool("force")
					if !force {
						fmt.Printf("Are you sure you want to delete identity with Peer ID: %s? [y/N]: ", peerIDStr)
						var response string
						_, sErr := fmt.Scanln(&response)
						if sErr != nil {
							log.Warn("Failed to read user input for confirmation", zap.Error(sErr))
							return sErr
						}
						response = strings.ToLower(strings.TrimSpace(response))
						if response != "y" && response != "yes" {
							fmt.Println("Delete operation aborted.")
							return nil
						}
					}

					if err := store.Delete(peerID); err != nil {
						log.Error("Failed to delete identity", zap.Error(err))
						return err
					}

					fmt.Printf("Identity with Peer ID: %s successfully deleted.\n\n", peerIDStr)
					return nil
				},
			},
		},
	}
}
