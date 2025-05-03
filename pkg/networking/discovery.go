// networking/discovery.go

package networking

import (
	"context"

	"github.com/unpackdev/fdb/pkg/logger"

	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	discovery "github.com/libp2p/go-libp2p/core/discovery"
	host "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	routing "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	observability "github.com/unpackdev/fdb/pkg/observability"
)

type DiscoveryServiceTag string

func (dst DiscoveryServiceTag) String() string {
	return string(dst)
}

// DiscoveryService encapsulates peer discovery mechanisms.
type DiscoveryService struct {
	ctx            context.Context
	DHT            *dht.IpfsDHT
	Discovery      *routing.RoutingDiscovery
	Logger         logger.Logger
	BootstrapPeers []peer.AddrInfo
	Metrics        *DiscoveryMetrics
	Observability  *observability.Observability
	bootstrapNode  bool
}

// NewDiscoveryService creates a new peer discovery service with a DHT and routing discovery mechanism.
// Parameters:
// - ctx: The context for managing cancellation and deadlines.
// - h: The libp2p host.
// - logger: The logger instance for logging events.
// - bootstrapNode: Indicates if this node is a bootstrap node.
// - bootstrapPeers: A list of bootstrap peers to connect to initially.
// - obs: The observability instance for metrics and tracing.
func NewDiscoveryService(
	ctx context.Context,
	h host.Host,
	logger logger.Logger,
	bootstrapNode bool,
	bootstrapPeers []peer.AddrInfo,
	obs *observability.Observability,
) (*DiscoveryService, error) {
	// Initialize Metrics using Observability's Meter
	metrics, err := InitializeDiscoveryMetrics(ctx, obs.Meter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize discovery metrics")
	}

	// Create a new Kademlia DHT instance with server mode enabled for better peer discovery.
	dhtInstance, err := dht.New(ctx, h, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create DHT instance")
	}

	// Create a routing-based discovery service.
	routingDiscovery := routing.NewRoutingDiscovery(dhtInstance)

	return &DiscoveryService{
		ctx:            ctx,
		DHT:            dhtInstance,
		Discovery:      routingDiscovery,
		Logger:         logger,
		BootstrapPeers: bootstrapPeers,
		Metrics:        metrics,
		Observability:  obs,
		bootstrapNode:  bootstrapNode,
	}, nil
}

// Start initializes the bootstrap process and connects to bootstrap peers.
func (ds *DiscoveryService) Start() error {
	// If bootstrap peers are provided and this node is not a bootstrap node, connect to them.
	if len(ds.BootstrapPeers) > 0 && !ds.bootstrapNode {
		// Add bootstrap peers to the peerstore.
		for _, peerInfo := range ds.BootstrapPeers {
			ds.DHT.Host().Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
		}

		// Connect to each bootstrap peer with retry logic.
		for _, peerInfo := range ds.BootstrapPeers {
			connected := false
			var err error
			for i := 0; i < 5; i++ { // Retry up to 5 times
				startTime := time.Now()
				err = ds.DHT.Host().Connect(ds.ctx, peerInfo)
				latency := time.Since(startTime)
				ds.Metrics.RecordDiscoveryLatency(ds.ctx, latency)

				if err == nil {
					ds.Logger.Info("Connected to bootstrap peer", zap.String("peer_id", peerInfo.ID.String()))
					ds.Metrics.RecordBootstrapPeersConnected(ds.ctx, 1)
					connected = true
					break
				}

				ds.Logger.Debug(
					"Failed to connect to bootstrap peer, retrying...",
					zap.Error(err),
					zap.String("peer_id", peerInfo.ID.String()),
				)
				ds.Metrics.RecordBootstrapPeersFailed(ds.ctx, 1)

				// Exponential backoff: 2^i seconds
				sleepDuration := time.Duration(1<<i) * time.Second
				time.Sleep(sleepDuration)
			}

			if !connected {
				ds.Logger.Error(
					"Exceeded maximum retries to connect to bootstrap peer",
					zap.Error(err),
					zap.String("peer_id", peerInfo.ID.String()),
				)
				return err
			}
		}
	} else {
		// No bootstrap peers provided. Relying on mDNS or direct connections.
		ds.Logger.Info("No bootstrap peers provided. Relying on mDNS or direct connections.")
	}

	// Bootstrap the DHT to join the network.
	if err := ds.DHT.Bootstrap(ds.ctx); err != nil {
		return errors.Wrap(err, "failed to bootstrap DHT")
	}

	ds.Logger.Info("Discovery service started successfully")
	return nil
}

// Advertise advertises the service with the given service tag.
// It allows bootstrap nodes to advertise without existing peers.
func (ds *DiscoveryService) Advertise(serviceTag DiscoveryServiceTag, isBootstrap bool) error {
	startTime := time.Now()
	for {
		select {
		case <-ds.ctx.Done():
			return nil
		default:
		}

		_, err := ds.Discovery.Advertise(ds.ctx, serviceTag.String(), discovery.TTL(10*time.Minute))
		if err != nil {
			if err.Error() == "failed to find any peer in table" {
				if isBootstrap {
					ds.Logger.Info("Bootstrap node, proceeding to advertise without existing peers")
					ds.Metrics.RecordPeersAdvertised(ds.ctx, 1)
					return nil
				}
				ds.Logger.Debug("No peers in DHT routing table yet. Retrying advertisement...")
				ds.Metrics.RecordPeersAdvertisementFailure(ds.ctx, 1)
				time.Sleep(5 * time.Second)
				continue
			}
			ds.Metrics.RecordPeersAdvertisementFailure(ds.ctx, 1)
			return errors.Wrap(err, "failed to advertise service")
		}

		ds.Logger.Info("Service advertised successfully", zap.String("service_tag", serviceTag.String()))
		ds.Metrics.RecordPeersAdvertised(ds.ctx, 1)
		latency := time.Since(startTime)
		ds.Metrics.RecordDiscoveryLatency(ds.ctx, latency)
		return nil
	}
}

// FindPeers discovers peers providing the given service.
// It returns a channel through which discovered peers are sent.
func (ds *DiscoveryService) FindPeers(serviceTag DiscoveryServiceTag) (<-chan peer.AddrInfo, error) {
	startTime := time.Now()
	peerChan, err := ds.Discovery.FindPeers(ds.ctx, serviceTag.String())
	if err != nil {
		ds.Metrics.RecordPeersDiscoveryFailure(ds.ctx, 1)
		return nil, errors.Wrap(err, "failed to find peers")
	}
	ds.Metrics.RecordPeersDiscovered(ds.ctx, 1)
	latency := time.Since(startTime)
	ds.Metrics.RecordDiscoveryLatency(ds.ctx, latency)
	return peerChan, nil
}

// AddBootstrapPeer allows adding new bootstrap peers to the DHT dynamically.
// It connects to the new bootstrap peer and records relevant metrics.
func (ds *DiscoveryService) AddBootstrapPeer(peerInfo peer.AddrInfo) error {
	ds.DHT.Host().Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
	startTime := time.Now()
	if err := ds.DHT.Host().Connect(ds.ctx, peerInfo); err != nil {
		ds.Logger.Warn("Failed to connect to bootstrap peer", zap.String("peerID", peerInfo.ID.String()), zap.Error(err))
		ds.Metrics.RecordBootstrapPeersFailed(ds.ctx, 1)
		ds.Metrics.RecordDiscoveryLatency(ds.ctx, time.Since(startTime))
		return errors.Wrapf(err, "failed to connect to bootstrap peer %s", peerInfo.ID.String())
	}
	ds.Logger.Info("Successfully added and connected to new bootstrap peer", zap.String("peerID", peerInfo.ID.String()))
	ds.Metrics.RecordBootstrapPeersConnected(ds.ctx, 1)
	ds.Metrics.RecordDiscoveryLatency(ds.ctx, time.Since(startTime))
	return nil
}

// RemovePeer removes a peer from the DHT routing table.
// It updates the relevant metrics to reflect the removal.
func (ds *DiscoveryService) RemovePeer(peerID peer.ID) {
	ds.DHT.Host().Peerstore().ClearAddrs(peerID)
	ds.DHT.RoutingTable().RemovePeer(peerID)
	ds.Logger.Info("Removed peer from routing table", zap.String("peerID", peerID.String()))
	ds.Metrics.RecordPeersRemoved(ds.ctx, 1)
	ds.Metrics.RecordActivePeers(ds.ctx, -1) // Decrement active peers by 1
}

// Shutdown gracefully shuts down the discovery service and cleans up resources.
// It ensures that the DHT is closed and any remaining peers are accounted for in metrics.
func (ds *DiscoveryService) Shutdown() error {
	ds.Logger.Info("Shutting down discovery service...")
	if err := ds.DHT.Close(); err != nil {
		// Assuming ListPeers() returns a slice of peer IDs currently in the routing table.
		activePeers := int64(len(ds.DHT.RoutingTable().ListPeers()))
		ds.Metrics.RecordPeersRemoved(ds.ctx, activePeers)
		ds.Metrics.RecordActivePeers(ds.ctx, -activePeers)
		return errors.Wrap(err, "failed to shut down DHT")
	}
	ds.Logger.Info("Discovery service shut down successfully")
	return nil
}
