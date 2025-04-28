// Package rpc provides an implementation of a JSON-RPC server over HTTP and WebSocket.
// It supports HTTP keep-alive connections and efficient message handling.
//
// The RPC server allows clients to make JSON-RPC calls over HTTP or WebSocket.
// It also supports subscription mechanisms over WebSocket, enabling real-time notifications.
//
// The package includes handlers for HTTP and WebSocket connections, a connection pool,
// and a server that manages registered RPC methods and dispatching requests to appropriate handlers.
//
// Example usage:
//
//	rpcServer := rpc.NewServer(logger, observability)
//	rpcServer.RegisterMethod("echo", EchoHandler)
//
//	// Start the RPC server
//	err := rpcServer.Start(context.Background())
//	if err != nil {
//	    log.Fatal("Failed to start RPC server:", err)
//	}
package rpc
