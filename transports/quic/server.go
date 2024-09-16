package transport_quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/pkg/errors"
	"github.com/quic-go/quic-go"
	"github.com/unpackdev/fdb/config"
	"github.com/unpackdev/fdb/messages"
	"github.com/unpackdev/fdb/types"
	"io"
	"log"
	"strings"
	"sync"
)

// QuicHandler function type for QUIC
type QuicHandler func(sess quic.Connection, stream quic.Stream, message *messages.Message)

// Server struct represents the QUIC server
type Server struct {
	ctx             context.Context
	handlerRegistry map[types.HandlerType]QuicHandler
	cnf             config.QuicTransport
	tlsConfig       *tls.Config
	stopChan        chan struct{}
	started         chan struct{}
	wg              sync.WaitGroup
	listener        *quic.Listener
}

// NewServer creates a new QuicServer instance
func NewServer(ctx context.Context, cnf config.QuicTransport) (*Server, error) {
	tlsConfig, tcErr := cnf.GetTLSConfig()
	if tcErr != nil {
		return nil, errors.Wrapf(tcErr, "could not get TLS config for quic transport")
	}

	server := &Server{
		ctx:             ctx,
		handlerRegistry: make(map[types.HandlerType]QuicHandler),
		cnf:             cnf,
		tlsConfig:       tlsConfig,
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the server address as a string.
func (s *Server) Addr() string {
	return s.cnf.Addr()
}

// Start starts the QUIC server
func (s *Server) Start() error {
	var err error
	s.listener, err = quic.ListenAddr(s.cnf.Addr(), s.tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to start QUIC server: %w", err)
	}

	// Signal that the server has started
	close(s.started)

	log.Printf("QUIC Server started on %s", s.cnf.Addr())

	s.wg.Add(1)
	go s.acceptConnections()
	return nil
}

// acceptConnections continuously accepts new QUIC connections
func (s *Server) acceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			conn, err := s.listener.Accept(context.Background())
			if err != nil {
				// Check if the error is a QUIC ApplicationError with code 0x0 (connection closed normally)
				var appErr *quic.ApplicationError
				if errors.As(err, &appErr) && appErr.ErrorCode == 0x0 {
					// Suppress logging for this specific error
					return
				}

				// Check if it's the specific "use of closed network connection" error
				if isClosedNetworkConnectionError(err) {
					// Suppress logging for this specific error
					return
				}

				// Log other errors
				log.Printf("Error accepting QUIC stream: %v", err)
				return // Exit if there's an error or the connection is closed
			}

			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

func (s *Server) handleConnection(conn quic.Connection) {
	defer s.wg.Done()

	// Continuously accept and handle new streams on the connection
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			// Check if the error is a QUIC ApplicationError with code 0x0 (connection closed normally)
			var appErr *quic.ApplicationError
			if errors.As(err, &appErr) && appErr.ErrorCode == 0x0 {
				// Suppress logging for this specific error
				return
			}

			// Check if it's the specific "use of closed network connection" error
			if isClosedNetworkConnectionError(err) {
				// Suppress logging for this specific error
				return
			}

			// Log other errors
			log.Printf("Error accepting QUIC stream: %v", err)
			return // Exit if there's an error or the connection is closed
		}

		s.wg.Add(1)
		go s.handleStream(conn, stream) // Handle each stream in a separate goroutine
	}
}

// isClosedNetworkConnectionError checks if the error is the "use of closed network connection" error.
func isClosedNetworkConnectionError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "server closed")
}

// handleStream handles incoming QUIC streams
func (s *Server) handleStream(conn quic.Connection, stream quic.Stream) {
	defer s.wg.Done()

	// Continuously read from the stream until it's closed
	for {
		// Step 1: Read from the stream into a buffer
		// Assuming max message size is known or stream EOF will signify message end
		buffer := make([]byte, 4096) // Adjust the buffer size based on your requirements
		n, err := stream.Read(buffer)
		if err != nil {
			// Check if the error is a QUIC ApplicationError with code 0x0 (connection closed normally)
			var appErr *quic.ApplicationError
			if errors.As(err, &appErr) && appErr.ErrorCode == 0x0 {
				// Suppress logging for this specific error
				return
			}

			// Handle specific "use of closed network connection" error
			if isClosedNetworkConnectionError(err) {
				return
			}

			// Handle other EOF or connection close errors
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, quic.ErrServerClosed) {
				return
			}

			// Log other errors
			log.Printf("Error reading from stream: %v", err)
			return
		}

		// Step 2: Decode the message from the buffer
		message, err := messages.Decode(buffer[:n])
		if err != nil {
			log.Printf("Error decoding message: %v", err)
			return
		}

		// Step 3: Log the received message
		//log.Printf("Received message: action=%d, key=%x, data=%s", message.Handler, message.Key, string(message.Data))

		// Step 4: Look up the appropriate handler for this action
		handler, exists := s.handlerRegistry[message.Handler]
		if !exists {
			log.Printf("No handler found for action type %d", message.Handler)
			return
		}

		// Step 5: Call the handler to process the message
		handler(conn, stream, message)
	}
}

// Stop stops the QUIC server
func (s *Server) Stop() error {
	close(s.stopChan)
	if err := s.listener.Close(); err != nil {
		return err
	}

	s.wg.Wait()
	return nil
}

// WaitStarted returns a channel that is closed when the server has started
func (s *Server) WaitStarted() <-chan struct{} {
	return s.started
}

// parseActionType parses the action type from the first byte of the stream
func (s *Server) parseActionType(frame []byte) (types.HandlerType, error) {
	if len(frame) < 1 {
		return 0, fmt.Errorf("invalid action: frame too short")
	}

	var actionType types.HandlerType
	err := actionType.FromByte(frame[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}

// RegisterHandler registers a handler for a specific action
func (s *Server) RegisterHandler(actionType types.HandlerType, handler QuicHandler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *Server) DeregisterHandler(actionType types.HandlerType) {
	delete(s.handlerRegistry, actionType)
}
