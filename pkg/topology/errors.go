// pkg/topology/errors.go
package topology

import (
	"errors"
)

// Custom errors for the topology package.
var (
	ErrPeerNotFound               = errors.New("peer not found in topology")
	ErrPeerAlreadyExists          = errors.New("peer already exists in topology")
	ErrInvalidMessageType         = errors.New("invalid message type")
	ErrInsufficientPermissions    = errors.New("insufficient permissions")
	ErrPeerNotInPendingPeers      = errors.New("peer not found in pendingPeers")
	ErrFailedToRemovePeer         = errors.New("failed to remove peer from topology")
	ErrFailedToProcessActorPacket = errors.New("failed to process actor packet")
	ErrUnknownActorPacketStatus   = errors.New("unknown ActorPacket status")
	ErrInvalidSignature           = errors.New("invalid signature")
	ErrTimeoutExceeded            = errors.New("wait operation timed out")
)
