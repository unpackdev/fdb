package metrics

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/unpackdev/fdb/pkg/logger"

	"go.uber.org/zap"
)

func PongHandler(logger logger.Logger) func(s network.Stream) {
	return func(s network.Stream) {
		// logger.Debug(
		// 	"Received new stream",
		// 	zap.String("protocol", string(s.Protocol())),
		// 	zap.String("from_peer_id", s.Conn().RemotePeer().String()),
		// )
		defer s.Close()

		buf := make([]byte, 1024)
		nBytes, err := s.Read(buf)
		if err != nil {
			logger.Warn("Error reading from stream", zap.Error(err))
			return
		}

		request := string(buf[:nBytes])
		// logger.Debug(
		// 	"Received request",
		// 	zap.String("request", request),
		// 	zap.String("from_peer_id", s.Conn().RemotePeer().String()),
		// )

		if request == "ping" {
			_, err := s.Write([]byte("pong"))
			if err != nil {
				logger.Warn("Error writing to stream", zap.Error(err))
			} else {
				//logger.Debug("Responded with pong", zap.String("to_peer_id", s.Conn().RemotePeer().String()))
			}
		}
	}
}
