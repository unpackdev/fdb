package transport_uds

import (
	"context"
	"github.com/panjf2000/gnet"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/types"
	"go.uber.org/zap"
	"os"
	"time"
)

// UDSHandler function type for UDS handlers
type UDSHandler func(c gnet.Conn, frame []byte)

// Server struct represents the Unix Domain Socket (UDS) server using gnet
type Server struct {
	*gnet.EventServer
	ctx             context.Context
	handlerRegistry map[types.HandlerType]UDSHandler
	cnf             config.UdsTransport
	stopChan        chan struct{}
	started         chan struct{}
}

// NewServer creates a new UDS Server instance using the provided configuration
func NewServer(ctx context.Context, cnf config.UdsTransport) (*Server, error) {
	// Remove the existing socket file if it exists
	if _, err := os.Stat(cnf.Socket); err == nil {
		if rmErr := os.Remove(cnf.Socket); rmErr != nil {
			return nil, errors.Wrap(rmErr, "failed to remove existing UDS socket file")
		}
	}

	server := &Server{
		ctx:             ctx,
		handlerRegistry: make(map[types.HandlerType]UDSHandler),
		cnf:             cnf,
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the UDS socket path as a string
func (s *Server) Addr() string {
	return s.cnf.Addr()
}

// Start starts the UDS server using the provided configuration
func (s *Server) Start() error {
	s.stopChan = make(chan struct{})
	s.started = make(chan struct{}) // Initialize the started channel
	listenAddr := "unix://" + s.cnf.Addr()
	zap.L().Info("Starting UDS Server", zap.String("addr", listenAddr))

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
		zap.L().Info("UDS Server successfully started", zap.String("addr", listenAddr))
		return nil
	case err := <-errChan:
		if err != nil {
			return errors.Wrap(err, "failed to start UDS server")
		}
		return nil
	case <-time.After(2 * time.Second): // Wait for up to 2 seconds
		return errors.New("UDS server did not start in time")
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

// Stop stops the UDS server
func (s *Server) Stop() error {
	zap.L().Info("Stopping UDS Server", zap.String("addr", s.cnf.Addr()))
	close(s.stopChan)

	if err := os.Remove(s.cnf.Addr()); err != nil {
		zap.L().Error("Failed to remove UDS socket file", zap.Error(err))
		return err
	}

	zap.L().Info("UDS socket file removed successfully", zap.String("addr", s.cnf.Addr()))
	return nil
}

// WaitStarted returns the started channel for waiting until the server starts
func (s *Server) WaitStarted() <-chan struct{} {
	return s.started
}

// OnInitComplete is called when the server starts
func (s *Server) OnInitComplete(server gnet.Server) (action gnet.Action) {
	zap.L().Info("UDS Server is listening", zap.String("addr", server.Addr.String()))
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
		zap.L().Warn("Failed to parse action type", zap.Error(err), zap.String("addr", c.RemoteAddr().String()))
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
func (s *Server) RegisterHandler(actionType types.HandlerType, handler UDSHandler) {
	zap.L().Debug("Registering handler", zap.Int("action_type", int(actionType)))
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *Server) DeregisterHandler(actionType types.HandlerType) {
	zap.L().Debug("Deregistering handler", zap.Int("action_type", int(actionType)))
	delete(s.handlerRegistry, actionType)
}
