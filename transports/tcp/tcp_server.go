package tcp

/*
import (
	"encoding/binary"
	"github.com/panjf2000/gnet"
	"log"
)

// TCPServer struct represents the TCP server
type TCPServer struct {
	*gnet.EventServer
	handlerRegistry map[HandlerType]Handler
	addr            string
}

// NewTCPServer creates a new TCPServer instance
func NewTCPServer(addr string) *TCPServer {
	return &TCPServer{
		handlerRegistry: make(map[HandlerType]Handler),
		addr:            addr,
	}
}

// RegisterHandler registers a handler for a specific action
func (s *TCPServer) RegisterHandler(actionType HandlerType, handler Handler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *TCPServer) DeregisterHandler(actionType HandlerType) {
	delete(s.handlerRegistry, actionType)
}

// Start starts the TCP server
func (s *TCPServer) Start() error {
	// Use LengthFieldBasedFrameCodec for message framing
	return gnet.Serve(s, s.addr, gnet.WithMulticore(true), gnet.WithCodec(gnet.LengthFieldBasedFrameCodec{
		EncoderConfig: gnet.EncoderConfig{
			ByteOrder:                       binary.BigEndian,
			LengthFieldLength:               4,
			LengthAdjustment:                0,
			LengthIncludesLengthFieldLength: false,
		},
		DecoderConfig: gnet.DecoderConfig{
			ByteOrder:           binary.BigEndian,
			LengthFieldOffset:   0,
			LengthFieldLength:   4,
			LengthAdjustment:    0,
			InitialBytesToStrip: 4,
		},
	}))
}

// OnInitComplete is called when the server starts
func (s *TCPServer) OnInitComplete(server gnet.Server) (action gnet.Action) {
	log.Printf("TCP Server started on %s", server.Addr.String())
	return
}

// React handles incoming data
func (s *TCPServer) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	if len(frame) < 1 {
		// Invalid frame
		log.Println("Received empty frame")
		return
	}

	actionType, err := s.parseActionType(frame)
	if err != nil {
		log.Printf("Error parsing action type: %v", err)
		return
	}

	handler, exists := s.handlerRegistry[actionType]
	if !exists {
		log.Printf("Handler not found for action type: %v", actionType)
		return
	}

	handler(c, frame)
	return
}

// parseActionType parses the action type from the frame
func (s *TCPServer) parseActionType(frame []byte) (HandlerType, error) {
	if len(frame) < 1 {
		return 0, errors.New("invalid action: frame too short")
	}

	return HandlerType(frame[0]), nil
}
*/
