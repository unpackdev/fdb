package config

import (
	"net"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

// Networking holds the configuration for the P2P networking.
type Networking struct {
	ListenAddrs    []string `yaml:"listenAddrs"`    // Addresses to listen on
	ProtocolID     string   `yaml:"protocolId"`     // Protocol ID for P2P communication
	PeerID         peer.ID  `yaml:"peerId"`         // PeerID for the node's identity (must be provided)
	BootstrapPeers []string `yaml:"bootstrapPeers"` // Bootstrap peers for discovery
	BootstrapNode  bool     `yaml:"bootstrapNode"`  // Indicates if this node should be a bootstrap node
	EnableMDNS     bool     `yaml:"mdns"`           // Enable mDNS for local peer discovery

	// Additional fields
	EnableRelay   bool   `yaml:"enableRelay"`    // Enable relay connections
	InterfaceName string `yaml:"interface_name"` // Network interface name for eBPF
}

// Validate checks the Networking configuration for required fields and correct formats.
func (n *Networking) Validate() error {
	// 1. Validate ListenAddrs
	if len(n.ListenAddrs) == 0 {
		return errors.New("networking.listenAddrs must have at least one address")
	}

	uniqueAddrs := make(map[string]bool)
	for _, addr := range n.ListenAddrs {
		// Check for uniqueness
		if uniqueAddrs[addr] {
			return errors.Errorf("duplicate listen address found: %s", addr)
		}
		uniqueAddrs[addr] = true

		// Validate multiaddress format
		_, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return errors.Wrapf(err, "invalid listen address: %s", addr)
		}
	}

	// 2. Validate ProtocolID
	if strings.TrimSpace(n.ProtocolID) == "" {
		return errors.New("networking.protocolId cannot be empty")
	}

	// 3. Validate PeerID (this must be provided for identity)
	if strings.TrimSpace(n.PeerID.String()) == "" {
		return errors.New("networking.peerId cannot be empty and must be provided")
	}

	// 4. Validate BootstrapPeers
	for _, peerAddr := range n.BootstrapPeers {
		// Validate multiaddress format
		_, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			return errors.Wrapf(err, "invalid bootstrap peer address: %s", peerAddr)
		}

		// Optionally, parse peer ID to ensure it's present
		ma, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			return errors.Wrapf(err, "invalid bootstrap peer multiaddress: %s", peerAddr)
		}
		_, err = peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			return errors.Wrapf(err, "bootstrap peer address must include a peer ID: %s", peerAddr)
		}
	}

	// 5. Validate InterfaceName (if provided)
	if strings.TrimSpace(n.InterfaceName) != "" {
		ifaces, err := net.Interfaces()
		if err != nil {
			return errors.Wrap(err, "failed to list network interfaces")
		}

		found := false
		for _, iface := range ifaces {
			if iface.Name == n.InterfaceName {
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("networking.interface_name does not match any existing network interface: %s", n.InterfaceName)
		}
	}

	return nil
}

func (c Config) IsBootstrapNode() bool {
	return c.Networking.BootstrapNode
}

// BootstrapPeersAsAddrs converts the BootstrapPeers slice into a slice of peer.AddrInfo objects.
func (n *Networking) BootstrapPeersAsAddrs() ([]peer.AddrInfo, error) {
	var addrInfos []peer.AddrInfo

	for _, peerAddr := range n.BootstrapPeers {
		// Convert the string to a multiaddress
		maddr, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse multiaddress: %s", peerAddr)
		}

		// Convert the multiaddress to peer.AddrInfo
		addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert multiaddress to peer.AddrInfo: %s", peerAddr)
		}

		addrInfos = append(addrInfos, *addrInfo)
	}

	return addrInfos, nil
}
