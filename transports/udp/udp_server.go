package fdb

import (
	"fmt"
	"github.com/panjf2000/gnet"
	"github.com/pkg/errors"
	"log"
	"net"
	"time"
)

// Handler function type
type Handler func(c gnet.Conn, frame []byte)

// UdpServer struct represents the UDP server using gnet
type UdpServer struct {
	*gnet.EventServer
	handlerRegistry map[HandlerType]Handler
	addr            *net.UDPAddr
	stopChan        chan struct{}
	started         chan struct{}
}

// NewUdpServer creates a new UdpServerGnet instance
func NewUdpServer(ip string, port int) (*UdpServer, error) {
	listenAddr := fmt.Sprintf("%s:%d", ip, port)
	netAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve address")
	}

	server := &UdpServer{
		handlerRegistry: make(map[HandlerType]Handler),
		addr:            netAddr,
	}

	return server, nil
}

func (s *UdpServer) Addr() *net.UDPAddr {
	return s.addr
}

// Start starts the UDP server
func (s *UdpServer) Start() error {
	s.stopChan = make(chan struct{})
	s.started = make(chan struct{}) // Initialize the started channel
	listenAddr := "udp://" + s.addr.String()
	log.Printf("UDP Server started on %s", listenAddr)
	return gnet.Serve(
		s, listenAddr,
		gnet.WithMulticore(true),
		gnet.WithReusePort(true),
		gnet.WithSocketRecvBuffer(1024*64),
		gnet.WithLockOSThread(true),
		gnet.WithTicker(true),
	)
}

// Tick is called periodically by gnet
func (s *UdpServer) Tick() (delay time.Duration, action gnet.Action) {
	select {
	case <-s.stopChan:
		return 0, gnet.Shutdown
	default:
		return time.Second, gnet.None
	}
}

// Stop stops the UDP server
func (s *UdpServer) Stop() {
	close(s.stopChan)
}

func (s *UdpServer) WaitStarted() <-chan struct{} {
	return s.started
}

// OnInitComplete is called when the server starts
func (s *UdpServer) OnInitComplete(server gnet.Server) (action gnet.Action) {
	log.Printf("UDP Server is listening on %s", server.Addr.String())
	close(s.started) // Signal that the server has started
	log.Println("")
	return
}

// React handles incoming data
/*func (s *UdpServer) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	if len(frame) < 1 {
		c.SendTo([]byte("ERROR: Invalid action"))
		return
	}

	actionType, err := s.parseActionType(frame)
	if err != nil {
		c.SendTo([]byte("ERROR: Invalid action"))
		return
	}

	handler, exists := s.handlerRegistry[actionType]
	if !exists {
		c.SendTo([]byte("ERROR: Unknown action"))
		return
	}

	// Call the handler
	handler(c, frame)

	return
}*/

func (s *UdpServer) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	/*	if len(frame) < 1 {
			// Directly return without calling SendTo inside React
			return []byte("ERROR: Invalid action"), gnet.None
		}

		// Use switch for faster action parsing
		actionType, err := s.parseActionType(frame)
		if err != nil {
			return []byte("ERROR: Invalid action"), gnet.None
		}

		// Check handler existence
		handler, exists := s.handlerRegistry[actionType]
		if !exists {
			return []byte("ERROR: Unknown action"), gnet.None
		}

		// Call the handler directly, no blocking operations
		handler(c, frame)*/

	// No output to send from React
	return nil, gnet.None
}

// parseActionType parses the action type from the frame
func (s *UdpServer) parseActionType(frame []byte) (HandlerType, error) {
	if len(frame) < 1 {
		return 0, errors.New("invalid action: frame too short")
	}

	var actionType HandlerType
	err := actionType.FromByte(frame[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}

// RegisterHandler registers a handler for a specific action
func (s *UdpServer) RegisterHandler(actionType HandlerType, handler Handler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *UdpServer) DeregisterHandler(actionType HandlerType) {
	delete(s.handlerRegistry, actionType)
}
