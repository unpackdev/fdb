// pkg/networking/discovery_test.go
package networking

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

// setupDiscoveryTest initializes the discovery service for two hosts and sets up the DHT.
func setupDiscoveryTest(t *testing.T, ctx context.Context, h1, h2 host.Host, logger *log.Logger) (*DiscoveryService, *DiscoveryService, error) {
	// Set each host as a bootstrap peer for the other.
	addBootstrapPeer(h1, h2, logger)
	addBootstrapPeer(h2, h1, logger)

	// Create DHT instances for each host.
	dht1, err := dht.New(ctx, h1, dht.Mode(dht.ModeServer))
	assert.NoError(t, err, "Failed to create DHT for host 1")

	dht2, err := dht.New(ctx, h2, dht.Mode(dht.ModeServer))
	assert.NoError(t, err, "Failed to create DHT for host 2")

	// Bootstrap the DHTs.
	err = dht1.Bootstrap(ctx)
	assert.NoError(t, err, "Failed to bootstrap DHT for host 1")

	err = dht2.Bootstrap(ctx)
	assert.NoError(t, err, "Failed to bootstrap DHT for host 2")

	// Force the DHTs to refresh their routing tables.
	logger.Println("Forcing DHT refresh for host 1...")
	<-dht1.ForceRefresh()
	assert.NoError(t, err, "Failed to refresh DHT for host 1")

	logger.Println("Forcing DHT refresh for host 2...")
	<-dht2.ForceRefresh()
	assert.NoError(t, err, "Failed to refresh DHT for host 2")

	// Create discovery services using the DHTs.
	discoveryService1, err := NewDiscoveryServiceWithDHT(ctx, h1, dht1, logger)
	assert.NoError(t, err, "Failed to create discovery service for host 1")

	discoveryService2, err := NewDiscoveryServiceWithDHT(ctx, h2, dht2, logger)
	assert.NoError(t, err, "Failed to create discovery service for host 2")

	return discoveryService1, discoveryService2, nil
}

// TestDiscoveryService performs a comprehensive test of the DiscoveryService.
func TestDiscoveryService(t *testing.T) {
	// Create a context for the test.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize logger.
	logger := log.New(os.Stdout, "DiscoveryTest: ", log.LstdFlags)

	// Create custom configurations for two libp2p hosts to avoid transport conflicts.
	listenAddr1, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/9001")
	listenAddr2, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/9002")

	// Create two libp2p hosts for testing the DHT and discovery service.
	h1, err := libp2p.New(libp2p.ListenAddrs(listenAddr1))
	assert.NoError(t, err, "Failed to create host 1")
	logger.Printf("Host 1 ID: %s", h1.ID())

	h2, err := libp2p.New(libp2p.ListenAddrs(listenAddr2))
	assert.NoError(t, err, "Failed to create host 2")
	logger.Printf("Host 2 ID: %s", h2.ID())

	// Set up the discovery services and DHT for both hosts.
	discoveryService1, discoveryService2, err := setupDiscoveryTest(t, ctx, h1, h2, logger)
	assert.NoError(t, err, "Failed to set up discovery services")

	// Wait for some time to allow the DHT to stabilize.
	time.Sleep(15 * time.Second)

	// Verify that peers are available in the routing table.
	assert.True(t, hasPeersInRoutingTable(discoveryService1), "No peers found in host 1's DHT")
	assert.True(t, hasPeersInRoutingTable(discoveryService2), "No peers found in host 2's DHT")

	// Advertise a test service using the discovery service on host 1.
	logger.Println("Advertising TestService on host 1...")
	err = discoveryService1.Advertise("TestService")
	assert.NoError(t, err, "Failed to advertise service")

	// Attempt to find peers providing the service from host 2.
	logger.Println("Finding TestService on host 2...")
	peerChan, err := discoveryService2.FindPeers("TestService")
	assert.NoError(t, err, "Failed to find peers")

	// Read from the peer channel to verify that a peer was found.
	select {
	case p := <-peerChan:
		logger.Printf("Discovered peer: %s", p.ID)
		assert.Equal(t, h1.ID(), p.ID, "Discovered peer ID should match host 1 ID")
	case <-time.After(10 * time.Second):
		t.Error("No peers found within the timeout period")
	}

	// Test adding a new bootstrap peer dynamically.
	logger.Println("Testing dynamic addition of bootstrap peer...")
	peerInfo := peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}
	err = discoveryService1.AddBootstrapPeer(peerInfo)
	assert.NoError(t, err, "Failed to add new bootstrap peer dynamically")

	// Remove the peer from the routing table.
	logger.Printf("Removing peer: %s", h2.ID())
	discoveryService1.RemovePeer(h2.ID())

	// Verify that the peer is removed from the routing table.
	assert.False(t, hasPeerInRoutingTable(discoveryService1, h2.ID()), "Peer should be removed from host 1's DHT")

	// Test service re-advertisement after peer removal.
	logger.Println("Re-advertising TestService after peer removal...")
	err = discoveryService1.Advertise("TestService")
	assert.NoError(t, err, "Failed to re-advertise service after peer removal")

	// Shutdown the discovery services and DHT instances.
	err = discoveryService1.Shutdown()
	assert.NoError(t, err, "Failed to shut down discovery service for host 1")

	err = discoveryService2.Shutdown()
	assert.NoError(t, err, "Failed to shut down discovery service for host 2")

	// Close the hosts.
	err = h1.Close()
	assert.NoError(t, err, "Failed to close host 1")

	err = h2.Close()
	assert.NoError(t, err, "Failed to close host 2")
}

// addBootstrapPeer adds a host as a bootstrap peer to another host.
func addBootstrapPeer(host, bootstrapPeer host.Host, logger *log.Logger) {
	host.Peerstore().AddAddrs(bootstrapPeer.ID(), bootstrapPeer.Addrs(), peerstore.PermanentAddrTTL)
	logger.Printf("Added bootstrap peer: %s", bootstrapPeer.ID())
}

// hasPeerInRoutingTable checks if a specific peer is present in the DHT's routing table.
func hasPeerInRoutingTable(service *DiscoveryService, peerID peer.ID) bool {
	for _, p := range service.DHT.RoutingTable().ListPeers() {
		if p == peerID {
			return true
		}
	}
	return false
}

// hasPeersInRoutingTable checks if the DHT has any peers in its routing table.
func hasPeersInRoutingTable(service *DiscoveryService) bool {
	return len(service.DHT.RoutingTable().ListPeers()) > 0
}
