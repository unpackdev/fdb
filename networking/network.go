package networking

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/unpackdev/fdb/accounts"

	"github.com/peerdns/peerd/pkg/privacy"
	"github.com/pkg/errors"
	"github.com/sasha-s/go-deadlock"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/observability"
	"github.com/unpackdev/fdb/packets"
	"github.com/unpackdev/fdb/state"
	"io"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// Message represents a message received from the network.
type Message struct {
	Data []byte
}

// Network defines the core structure of the P2P network.
type Network struct {
	host            host.Host
	pubsub          *pubsub.PubSub
	Topic           *pubsub.Topic
	ProtocolID      protocol.ID
	ctx             context.Context
	cancel          context.CancelFunc
	discovery       *DiscoveryService
	cfg             config.Networking // Store the configuration in the P2PNetwork struct
	mu              deadlock.RWMutex
	Logger          logger.Logger
	PrivacyManager  *privacy.PrivacyManager
	mdns            mdns.Service
	mdnsNotifier    *MdnsNotifier
	Metrics         *P2PMetrics
	Observability   *observability.Observability
	pubSubService   *PubSubService
	stateMgr        *state.StateManager
	handlerRegistry *PacketHandlerRegistry
	notifier        *Notifier
}

// NewNetwork initializes and returns a new P2P network without starting it.
func NewNetwork(ctx context.Context, cfg config.Networking, account *accounts.Account, bootstrapAddrs []peer.AddrInfo, logger logger.Logger, obs *observability.Observability, stateMgr *state.StateManager) (*Network, error) {
	stateMgr.SetState(NetworkStateType, state.Initializing)

	ctx, cancel := context.WithCancel(ctx)

	// Create the libp2p host and node. This function will load actual host and then
	// initiate and prepare libp2p node. Will not start the node or any type of discovery.
	libp2pHost, err := CreateNode(cfg, logger, account)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	metrics, err := InitializeP2PMetrics(ctx, obs.Meter)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize P2P metrics: %w", err)
	}

	discoveryService, err := NewDiscoveryService(ctx, libp2pHost, logger, cfg.BootstrapNode, bootstrapAddrs, obs)
	if err != nil {
		logger.Error("Failed to initialize discovery service", zap.Error(err))
		cancel()
		return nil, err
	}

	// Initialize global peer connectivity notifier manager
	notifier := NewNotifier(logger)

	// Initialize Message Handler Registry
	handlerRegistry := NewMessageHandlerRegistry(logger)

	toReturn := &Network{
		ctx:             ctx,
		cancel:          cancel,
		host:            libp2pHost,
		discovery:       discoveryService,
		ProtocolID:      protocol.ID(cfg.ProtocolID), // Use ProtocolID from config
		cfg:             cfg,
		Logger:          logger,
		Metrics:         metrics,
		Observability:   obs,
		stateMgr:        stateMgr,
		handlerRegistry: handlerRegistry,
		notifier:        notifier,
	}

	// Set up peer event handlers using network.NotifyBundle
	toReturn.host.Network().Notify(&network.NotifyBundle{
		ConnectedF:    toReturn.handlePeerConnected,
		DisconnectedF: toReturn.handlePeerDisconnected,
	})

	// Set the stream handler for the custom protocol.
	toReturn.host.SetStreamHandler(toReturn.ProtocolID, toReturn.handleStream)

	// Setting up the mdns notifier here so we can tweak it prior any service starts the process
	toReturn.mdnsNotifier = &MdnsNotifier{n: toReturn}

	ps, err := pubsub.NewGossipSub(ctx, libp2pHost)
	if err != nil {
		stateMgr.SetState(NetworkStateType, state.Failed)
		logger.Error("Failed to create PubSub", zap.Error(err))
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}
	toReturn.pubsub = ps
	topic, err := ps.Join(cfg.ProtocolID)
	if err != nil {
		stateMgr.SetState(NetworkStateType, state.Failed)
		logger.Error("Failed to join topic", zap.Error(err))
		return nil, fmt.Errorf("failed to join topic: %w", err)
	}

	toReturn.Topic = topic
	toReturn.pubSubService = NewPubSubService(ctx, ps)

	stateMgr.SetState(NetworkStateType, state.Initialized)

	return toReturn, nil
}

func (n *Network) Host() host.Host {
	return n.host
}

func (n *Network) PubSubService() *PubSubService {
	return n.pubSubService
}

func (n *Network) Discovery() *DiscoveryService {
	return n.discovery
}

func (n *Network) MdnsNotifier() *MdnsNotifier {
	return n.mdnsNotifier
}

func (n *Network) HandlerRegistry() *PacketHandlerRegistry {
	return n.handlerRegistry
}

// Start begins the P2P network's operations.
func (n *Network) Start() error {
	n.stateMgr.SetState(NetworkStateType, state.Starting)

	// Set up mDNS discovery if enabled.
	if n.cfg.EnableMDNS {
		if err := n.StartMDNS(); err != nil {
			return errors.Wrap(err, "failed to start MDNS service")
		}
	}

	// Starts the discovery service
	if err := n.discovery.Start(); err != nil {
		n.stateMgr.SetState(NetworkStateType, state.Failed)
		return errors.Wrap(err, "failed to start P2P discovery service")
	}

	// Join or create a topic using ProtocolID from the configuration.
	n.Logger.Debug("Network joining topic", zap.String("topic", n.cfg.ProtocolID))

	// Log the host information.
	n.Logger.Info("P2P network started",
		zap.String("protocol_id", string(n.ProtocolID)),
		zap.Strings("listen_addresses", addrStrings(n.host.Addrs())),
		zap.String("host_id", n.host.ID().String()),
	)

	// Connect to bootstrap peers if provided and if current node is not bootstrap node itself.
	// Bootstrap node has no other nodes to connect to as it is the first node in the stack.
	if !n.cfg.BootstrapNode {
		for _, peerAddr := range n.cfg.BootstrapPeers {
			if pErr := n.ConnectPeer(peerAddr); pErr != nil {
				n.Logger.Warn("Failed to connect to bootstrap peer", zap.String("peer", peerAddr), zap.Error(pErr))
			} else {
				n.Logger.Info("Connected to bootstrap peer", zap.String("peer", peerAddr))
			}
		}
	}

	n.stateMgr.SetState(NetworkStateType, state.Started)

	return nil
}

func (n *Network) StartMDNS() error {
	mdnsService := mdns.NewMdnsService(n.host, "peerdns-mdns", n.mdnsNotifier)
	if mErr := mdnsService.Start(); mErr != nil {
		n.stateMgr.SetState(NetworkStateType, state.Failed)
		n.Logger.Error("Failed to start mDNS service", zap.Error(mErr))
		return mErr
	} else {
		n.mdns = mdnsService
		n.Logger.Info("mDNS service started")
	}
	return nil
}

// Shutdown gracefully shuts down the P2P network.
func (n *Network) Shutdown() error {
	n.stateMgr.SetState(NetworkStateType, state.Stopping)
	n.Logger.Info("Shutting down P2P network")
	n.cancel()

	// Close the mDNS service if it was started.
	if n.mdns != nil {
		if err := n.mdns.Close(); err != nil {
			n.stateMgr.SetState(NetworkStateType, state.Failed)
			return errors.Wrap(err, "failed to close mdns service")
		} else {
			n.Logger.Debug("mDNS service closed successfully")
		}
	}

	// Close the host and release resources.
	if err := n.host.Close(); err != nil {
		n.stateMgr.SetState(NetworkStateType, state.Failed)
		return errors.Wrap(err, "failed to shut down network host")
	} else {
		n.Logger.Debug("Host closed successfully")
	}

	n.stateMgr.SetState(NetworkStateType, state.Stopped)
	return nil
}

// handleStream processes incoming streams using the custom protocol with length-prefixing.
func (n *Network) handleStream(s network.Stream) {
	defer s.Close()

	peerID := s.Conn().RemotePeer()
	n.Logger.Info("Received stream from peer", zap.String("peer_id", peerID.String()))

	// Prevent processing messages from self
	if peerID == n.host.ID() {
		n.Logger.Warn("Received stream from self. Ignoring.", zap.String("peer_id", peerID.String()))
		return
	}

	reader := bufio.NewReader(s)

	for {
		// Read the length prefix
		var packetLength uint32
		err := binary.Read(reader, binary.LittleEndian, &packetLength)
		if err != nil {
			if err == io.EOF {
				// Connection closed by peer
				n.Logger.Info("Connection closed by peer", zap.String("peer_id", peerID.String()))
			} else {
				n.Logger.Error("Failed to read packet length", zap.Error(err), zap.String("peerID", peerID.String()))
			}
			return
		}

		// Read the packet data based on the length
		message := make([]byte, packetLength)
		_, err = io.ReadFull(reader, message)
		if err != nil {
			n.Logger.Error("Failed to read complete packet", zap.Error(err), zap.String("peerID", peerID.String()))
			return
		}

		startTime := time.Now()

		// Deserialize the NetworkPacket
		networkPacket, npErr := packets.DeserializeNetworkPacket(message)
		if npErr != nil {
			n.Logger.Error("Failed to deserialize network packet", zap.Error(npErr))
			return
		}

		// TODO: It should always require signature public key...
		// For now to get it working at least somehow.
		if len(networkPacket.SignaturePubKey) > 0 {
			networkPacketData, _ := networkPacket.SerializeWithoutSignature()
			if vfErr := VerifySignature(networkPacket.SignaturePubKey, networkPacketData, networkPacket.Signature); vfErr != nil {
				n.Logger.Error("Received invalid network packet signature",
					zap.Error(vfErr),
					zap.String("from_peer", peerID.String()),
					zap.String("packet_type", networkPacket.Type.String()),
					zap.ByteString("payload", networkPacket.Payload),
				)
				return
			}
		}

		n.Logger.Debug("Received message",
			zap.String("from_peer", peerID.String()),
			zap.String("packet_type", networkPacket.Type.String()),
			zap.ByteString("payload", networkPacket.Payload),
		)
		n.Metrics.RecordMessagesReceived(n.ctx, 1)
		n.Metrics.RecordMessageLatency(n.ctx, time.Since(startTime))

		// Invoke the appropriate handler
		if err := n.handlerRegistry.HandlePacket(n.ctx, networkPacket, peerID); err != nil {
			n.Logger.Error("HandleMessage failed", zap.Error(err))
		}
	}
}

// ConnectPeerInfo connects to a peer using the provided peer.AddrInfo.
func (n *Network) ConnectPeerInfo(pi peer.AddrInfo) error {
	startTime := time.Now()
	if err := n.host.Connect(n.ctx, pi); err != nil {
		n.Logger.Debug("Failed to connect to peer", zap.String("peer_id", pi.ID.String()), zap.Error(err))
		n.Metrics.RecordPeersConnectionFailed(n.ctx, 1)
		n.Metrics.RecordMessageLatency(n.ctx, time.Since(startTime))
		return fmt.Errorf("failed to connect to peer %s: %w", pi.ID, err)
	}

	n.Logger.Info("Connected to peer",
		zap.String("peer_id", pi.ID.String()),
		zap.Strings("addresses", addrStrings(pi.Addrs)),
	)
	n.Metrics.RecordPeersConnected(n.ctx, 1)
	n.Metrics.RecordActivePeers(n.ctx, 1)
	n.Metrics.RecordMessageLatency(n.ctx, time.Since(startTime))

	return nil
}

// ConnectPeer connects to a given peer using its multiaddress.
func (n *Network) ConnectPeer(peerAddr string) error {
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(n.ctx, 1)
		return fmt.Errorf("invalid multiaddress: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(n.ctx, 1)
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	n.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

	startTime := time.Now()
	if err := n.host.Connect(n.ctx, *peerInfo); err != nil {
		n.Logger.Warn("Failed to connect to peer", zap.String("peer_id", peerInfo.ID.String()), zap.Error(err))
		n.Metrics.RecordPeersConnectionFailed(n.ctx, 1)
		n.Metrics.RecordMessageLatency(n.ctx, time.Since(startTime))
		return fmt.Errorf("failed to connect to peer %s: %w", peerInfo.ID, err)
	}

	n.Logger.Info("Connected to peer",
		zap.String("peer_id", peerInfo.ID.String()),
		zap.Strings("addresses", addrStrings(peerInfo.Addrs)),
	)
	n.Metrics.RecordPeersConnected(n.ctx, 1)
	n.Metrics.RecordActivePeers(n.ctx, 1)
	n.Metrics.RecordMessageLatency(n.ctx, time.Since(startTime))

	return nil
}

// SendMessage sends a direct message to a specific peer using the custom protocol.
// It prepends the message with its length as a uint32 in little-endian format.
func (n *Network) SendMessage(ctx context.Context, protocolId protocol.ID, target peer.ID, message []byte) error {
	// Prevent sending messages to self
	if target == n.host.ID() {
		n.Logger.Debug("Attempted to send message to self. Ignoring.", zap.String("peer_id", target.String()))
		return nil
	}

	startTime := time.Now()

	// Prepare the message with a length prefix
	var buffer bytes.Buffer
	packetLength := uint32(len(message))
	if err := binary.Write(&buffer, binary.LittleEndian, packetLength); err != nil {
		n.Logger.Error("Failed to write packet length", zap.Error(err))
		return fmt.Errorf("failed to write packet length: %w", err)
	}
	if _, err := buffer.Write(message); err != nil {
		n.Logger.Error("Failed to write message to buffer", zap.Error(err))
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Open a new stream to the target peer
	stream, err := n.host.NewStream(ctx, target, protocolId)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(ctx, 1)
		n.Metrics.RecordMessageLatency(ctx, time.Since(startTime))
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	// Write the length-prefixed message to the stream
	_, err = stream.Write(buffer.Bytes())
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(ctx, 1)
		n.Metrics.RecordMessageLatency(ctx, time.Since(startTime))
		return fmt.Errorf("failed to write message: %w", err)
	}

	n.Logger.Debug(
		"Sent P2P message",
		zap.String("to_peer", target.String()),
		zap.Any("protocol", protocolId),
		zap.ByteString("message", message),
	)
	n.Metrics.RecordMessagesSent(ctx, 1)
	n.Metrics.RecordMessageLatency(ctx, time.Since(startTime))

	return nil
}

// BroadcastMessage sends a message to all peers subscribed to the PubSub topic.
func (n *Network) BroadcastMessage(message []byte) error {
	startTime := time.Now()
	err := n.Topic.Publish(n.ctx, message)
	if err != nil {
		n.Metrics.RecordPeersConnectionFailed(n.ctx, 1)
		n.Metrics.RecordMessageLatency(n.ctx, time.Since(startTime))
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	n.Logger.Info("Broadcasted message", zap.ByteString("message", message))
	n.Metrics.RecordMessagesSent(n.ctx, 1)
	n.Metrics.RecordMessageLatency(n.ctx, time.Since(startTime))

	return nil
}

// BroadcastPacketOverTopic sends a message to all peers subscribed to a specific PubSub topic.
func (n *Network) BroadcastPacketOverTopic(ctx context.Context, topic *pubsub.Topic, message []byte) error {
	err := topic.Publish(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	n.Logger.Debug(
		"Broadcasted packet",
		zap.ByteString("message", message),
		zap.String("protocol", topic.String()),
	)

	return nil
}
