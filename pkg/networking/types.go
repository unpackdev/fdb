package networking

import (
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/unpackdev/fdb/pkg/state"
)

type TopicType string

func (t TopicType) String() string {
	return string(t)
}

func (t TopicType) AsProtocolID() protocol.ID {
	return protocol.ID(t.String())
}

var (
	NetworkStateType state.StateType = "networking"
)
