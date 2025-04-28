package websocket

import (
	"bytes"
	"errors"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/panjf2000/gnet/v2"
	"github.com/unpackdev/fdb/logger"
	"go.uber.org/zap"
	"io"
)

// readWriteWrapper combines gnet.Conn read and write capabilities to implement io.ReadWriter.
type readWriteWrapper struct {
	reader io.Reader
	conn   gnet.Conn
}

func (rw readWriteWrapper) Read(p []byte) (n int, err error) {
	return rw.reader.Read(p)
}

func (rw readWriteWrapper) Write(p []byte) (n int, err error) {
	return rw.conn.Write(p)
}

// WsCodec handles WebSocket upgrades and frame decoding.
type WsCodec struct {
	upgraded bool // Indicates if the connection has been upgraded
	logger   logger.Logger
	buf      bytes.Buffer // Buffer to accumulate incoming data for handshake and messages
	wsMsgBuf wsMessageBuf // Buffer to handle fragmented WebSocket messages
}

func NewWsCodec(logger logger.Logger) *WsCodec {
	return &WsCodec{
		logger: logger,
	}
}

func (w *WsCodec) Upgraded() bool {
	return w.upgraded
}

// wsMessageBuf holds state for the current WebSocket message being read.
type wsMessageBuf struct {
	curHeader *ws.Header
	cachedBuf bytes.Buffer
}

func (w *WsCodec) Upgrade(c gnet.Conn) (bool, gnet.Action) {
	if w.upgraded {
		return true, gnet.None
	}

	// Accumulate data in the buffer
	data, err := c.Next(-1)
	if err != nil {
		w.logger.Error("Failed to read handshake data", zap.Error(err))
		return false, gnet.Close
	}
	w.buf.Write(data)

	// Attempt to perform the upgrade using gobwas/ws Upgrade function
	bufReader := bytes.NewReader(w.buf.Bytes())
	readWriter := readWriteWrapper{
		reader: bufReader,
		conn:   c,
	}

	hs, err := ws.Upgrade(readWriter)
	skipN := len(w.buf.Bytes()) - bufReader.Len() // Calculate how much data was read
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			// Incomplete handshake, waiting for more data
			w.logger.Debug("WebSocket handshake incomplete, waiting for more data", zap.Error(err))
			return false, gnet.None
		}
		// Log the error and close the connection
		w.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return false, gnet.Close
	}

	// Successfully upgraded, remove consumed handshake data
	w.buf.Next(skipN)
	w.logger.Info("WebSocket connection upgraded",
		zap.String("remote_addr", c.RemoteAddr().String()),
		zap.Any("handshake", hs),
	)
	w.upgraded = true
	return true, gnet.None
}

func (w *WsCodec) ReadBufferBytes(c gnet.Conn) gnet.Action {
	// Read buffered data from the connection
	size := c.InboundBuffered()
	buf := make([]byte, size)
	read, err := c.Read(buf)
	if err != nil {
		w.logger.Error("Failed to read buffer bytes", zap.Error(err))
		return gnet.Close
	}
	if read < size {
		w.logger.Error("Read bytes length mismatch", zap.Int("size", size), zap.Int("read", read))
		return gnet.Close
	}
	w.buf.Write(buf)
	return gnet.None
}

func (w *WsCodec) Decode(c gnet.Conn) ([]wsutil.Message, error) {
	var messages []wsutil.Message
	for {
		// Use the internal buffer to read WebSocket frames
		if w.buf.Len() < ws.MinHeaderSize {
			// Not enough data for a complete header
			return messages, nil
		}

		var head ws.Header
		tmpReader := bytes.NewReader(w.buf.Bytes())
		oldLen := tmpReader.Len()

		// Read header
		head, err := ws.ReadHeader(tmpReader)
		if err != nil {
			if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
				// Data is incomplete, wait for more
				return messages, nil
			}
			return nil, err
		}
		skipN := oldLen - tmpReader.Len()
		w.buf.Next(skipN)

		// Read payload based on the header length
		dataLen := int(head.Length)
		if w.buf.Len() < dataLen {
			// Not enough data for the full payload, wait for more
			return messages, nil
		}

		payload := make([]byte, dataLen)
		_, err = io.ReadFull(&w.buf, payload)
		if err != nil {
			return nil, err
		}

		// Unmask the payload if it's masked (client-to-server)
		if head.Masked {
			ws.Cipher(payload, head.Mask, 0)
		}

		// Handle control messages separately
		if head.OpCode.IsControl() {
			err = wsutil.HandleClientControlMessage(c, wsutil.Message{
				OpCode:  head.OpCode,
				Payload: payload,
			})
			if err != nil {
				return nil, err
			}
			continue
		}

		// Store the message
		messages = append(messages, wsutil.Message{
			OpCode:  head.OpCode,
			Payload: payload,
		})

		// Check if the current message is finished
		if head.Fin {
			break
		}
	}
	return messages, nil
}
