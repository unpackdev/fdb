package fdb

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/pkg/errors"
	"github.com/quic-go/quic-go"
	"io"
	"log"
	"strings"
	"sync"
)

// Handler function type for QUIC
type QuicHandler func(sess quic.Connection, stream quic.Stream, message *Message)

// QuicServer struct represents the QUIC server
type QuicServer struct {
	handlerRegistry map[HandlerType]QuicHandler
	addr            string
	tlsConfig       *tls.Config
	stopChan        chan struct{}
	started         chan struct{}
	wg              sync.WaitGroup
	listener        *quic.Listener
}

// NewQuicServer creates a new QuicServer instance
func NewQuicServer(ip string, port int, tlsConfig *tls.Config) (*QuicServer, error) {
	listenAddr := fmt.Sprintf("%s:%d", ip, port)

	server := &QuicServer{
		handlerRegistry: make(map[HandlerType]QuicHandler),
		addr:            listenAddr,
		tlsConfig:       tlsConfig,
		stopChan:        make(chan struct{}),
		started:         make(chan struct{}),
	}

	return server, nil
}

// Addr returns the server address as a string.
func (s *QuicServer) Addr() string {
	return s.addr
}

// Start starts the QUIC server
func (s *QuicServer) Start() error {
	var err error
	s.listener, err = quic.ListenAddr(s.addr, s.tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to start QUIC server: %w", err)
	}

	// Signal that the server has started
	close(s.started)

	log.Printf("QUIC Server started on %s", s.addr)

	s.wg.Add(1)
	go s.acceptConnections()

	return nil
}

// acceptConnections continuously accepts new QUIC connections
func (s *QuicServer) acceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			conn, err := s.listener.Accept(context.Background())
			if err != nil {
				log.Printf("Error accepting QUIC connection: %v", err)
				continue
			}

			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

func (s *QuicServer) handleConnection(conn quic.Connection) {
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
	// Check for the specific error message
	return strings.Contains(err.Error(), "use of closed network connection")
}

// handleStream handles incoming QUIC streams
func (s *QuicServer) handleStream(conn quic.Connection, stream quic.Stream) {
	defer s.wg.Done()

	// Continuously read from the stream until it's closed
	for {
		// Step 1: Read from the stream into a buffer
		// Assuming max message size is known or stream EOF will signify message end
		buffer := make([]byte, 4096) // Adjust the buffer size based on your requirements
		n, err := stream.Read(buffer)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			log.Printf("Error reading from stream: %v", err)
			return
		}

		// Step 2: Decode the message from the buffer
		message, err := Decode(buffer[:n])
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
func (s *QuicServer) Stop() {
	close(s.stopChan)
	s.listener.Close()
	s.wg.Wait()
}

// WaitStarted returns a channel that is closed when the server has started
func (s *QuicServer) WaitStarted() <-chan struct{} {
	return s.started
}

// parseActionType parses the action type from the first byte of the stream
func (s *QuicServer) parseActionType(frame []byte) (HandlerType, error) {
	if len(frame) < 1 {
		return 0, fmt.Errorf("invalid action: frame too short")
	}

	var actionType HandlerType
	err := actionType.FromByte(frame[0])
	if err != nil {
		return 0, err
	}

	return actionType, nil
}

// RegisterHandler registers a handler for a specific action
func (s *QuicServer) RegisterHandler(actionType HandlerType, handler QuicHandler) {
	s.handlerRegistry[actionType] = handler
}

// DeregisterHandler deregisters a handler for a specific action
func (s *QuicServer) DeregisterHandler(actionType HandlerType) {
	delete(s.handlerRegistry, actionType)
}
