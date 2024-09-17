package client

import (
	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

var messageRegistry = map[MessageType]func(c gnet.Conn, data []byte) error{
	InvalidActionMessageType: func(c gnet.Conn, data []byte) error {
		zap.L().Error(
			"Invalid action....",
			zap.String("action", string(data)),
		)
		return nil
	},
}
