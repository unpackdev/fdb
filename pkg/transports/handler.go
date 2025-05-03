// pkg/transports/handler.go
package transports

// Handler defines the interface for handling incoming messages.
type Handler interface {
	Handle(conn Connection, frame []byte)
}

// HandlerFunc is a helper to allow functions to be used as handlers.
type HandlerFunc func(conn Connection, frame []byte)

// Handle calls the handler function.
func (f HandlerFunc) Handle(conn Connection, frame []byte) {
	f(conn, frame)
}
