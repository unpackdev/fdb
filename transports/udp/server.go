package transport_udp

import (
	"context"
	"github.com/panjf2000/gnet"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"time"
)

// UDPHandler function type for UDP handlers
type UDPHandler func(c gnet.Conn, frame []byte)

// Server struct represents the UDP server using gnet
type Server struct {
	*gnet.EventServer
	ctx             context.Context
	handlerRegistry map[types.HandlerType]UDPHandler
	cnf             config.UdpTransport
	stopChan        chan struct{}
	started         chan struct{}
}

// NewServer creates a new UDP Server instance using the provided configuration
func NewServer(ctx context.Context, cnf config.UdpTransport) (*Server, error) {
	server := &Server{
		ctx:             ctx,
		handlerRegistry: make(map[types.HandlerType]UDPHandler),
		cnf:             cnf,
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the UDP address as a string
func (s *Server) Addr() string {
	return s.cnf.Addr()
}

// Start starts the UDP server using the provided configuration
func (s *Server) Start() error {
	s.stopChan = make(chan struct{})
	s.started = make(chan struct{}) // Initialize the started channel
	listenAddr := "udp://" + s.cnf.Addr()
	zap.L().Info("Starting UDP Server", zap.String("addr", listenAddr))

	// Create an error channel to capture errors from the goroutine
	errChan := make(chan error, 1)

	// Start the server asynchronously
	go func() {
		err := gnet.Serve(
			s, listenAddr,
			gnet.WithMulticore(true),
			gnet.WithReusePort(true),
			gnet.WithSocketRecvBuffer(1024*64),
			gnet.WithLockOSThread(true),
			gnet.WithTicker(true),
		)
		if err != nil {
			errChan <- err
			return
		}
		close(errChan) // No error, close the channel
	}()

	// Wait until OnInitComplete sends a signal or an error occurs
	select {
	case <-s.started:
		close(s.started)
		zap.L().Info("UDP Server successfully started", zap.String("addr", listenAddr))
		return nil
	case err := <-errChan:
		if err != nil {
			return errors.Wrap(err, "failed to start UDP server")
		}
		return nil
	case <-time.After(2 * time.Second): // Wait for up to 2 seconds
		return errors.New("UDP server did not start in time")
	}
}

// Tick is called periodically by gnet
func (s *Server) Tick() (delay time.Duration, action gnet.Action) {
	select {
	case <-s.stopChan:
		return 0, gnet.Shutdown
	default:
		return time.Second, gnet.None
	}
}

// Stop stops the UDP server
func (s *Server) Stop() error {
	zap.L().Info("Stopping UDP Server", zap.String("addr", s.cnf.Addr()))
	close(s.stopChan)

	zap.L().Info("UDP Server stopped successfully", zap.String("addr", s.cnf.Addr()))
	return nil
}

// WaitStarted returns the started channel for waiting until the server starts
func (s *Server) WaitStarted() <-chan struct{} {
	return s.started
}

// OnInitComplete is called when the server starts
func (s *Server) OnInitComplete(server gnet.Server) (action gnet.Action) {
	zap.L().Info("UDP Server is listening", zap.String("addr", server.Addr.String()))
	s.started <- struct{}{} // Signal that the server has started
	return gnet.None
}

// React handles incoming data
func (s *Server) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	if len(frame) < 1 {
		zap.L().Warn("Invalid action received", zap.String("addr", c.RemoteAddr().String()))
		return []byte("ERROR: Invalid action"), gnet.None
	}

	// Parse the action type
	actionType, err := s.parseActionType(frame)
	if err != nil {
		//zap.L().Warn("Failed to parse action type", zap.Error(err), zap.String("addr", c.RemoteAddr().String()))
		return []byte("ERROR: Invalid action"), gnet.None
	}

	// Check if the handler exists
	handler, exists := s.handlerRegistry[actionType]
	if !exists {
		zap.L().Warn("Unknown action type", zap.Int("action_type", int(actionType)), zap.String("addr", c.RemoteAddr().String()))
		return []byte("ERROR: Unknown action"), gnet.None
	}

	// Call the handler
	handler(c, frame)
	return nil, gnet.None
}

// parseActionType parses the action type from the frame
func (s *Server) parseActionType(frame []byte) (types.HandlerType, error) {
	if len(frame) < 1 {
		return 0, errors.New("invalid action: frame too short")
	}

	var actionType types.HandlerType
	err := actionType.FromByte(frame[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}

// RegisterHandler registers a handler for a specific action
func (s *Server) RegisterHandler(actionType types.HandlerType, handler UDPHandler) {
	zap.L().Debug("Registering handler", zap.Int("action_type", int(actionType)))
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *Server) DeregisterHandler(actionType types.HandlerType) {
	zap.L().Debug("Deregistering handler", zap.Int("action_type", int(actionType)))
	delete(s.handlerRegistry, actionType)
}
