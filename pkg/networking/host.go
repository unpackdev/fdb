package networking

import (
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/unpackdev/fdb/pkg/accounts"
	"github.com/unpackdev/fdb/pkg/config"
	"github.com/unpackdev/fdb/pkg/logger"
	"go.uber.org/zap"
)

// CreateNode initializes a libp2p host and node based on the provided configuration.
func CreateNode(cfg config.Networking, logger logger.Logger, account *accounts.Account) (host.Host, error) {
	// Validate the networking configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid networking configuration: %w", err)
	}

	// Parse and validate the listen addresses from the configuration.
	listenAddrs, err := parseListenAddresses(cfg.ListenAddrs)
	if err != nil {
		return nil, fmt.Errorf("invalid listen addresses: %w", err)
	}

	hostOptions := []libp2p.Option{
		libp2p.NATPortMap(),
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.Identity(account.MasterPrivateKey()),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		//libp2p.NoSecurity, // Disable security for testing
	}

	// Add additional options based on configuration (e.g., Relay, etc.)
	if cfg.EnableRelay {
		hostOptions = append(hostOptions, libp2p.EnableRelay())
	}

	libp2pHost, err := libp2p.New(hostOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Log the host information.
	logger.Info("Libp2p host created",
		zap.String("hostID", libp2pHost.ID().String()),
		zap.Strings("listenAddresses", addrStrings(libp2pHost.Addrs())),
	)

	return libp2pHost, nil
}
