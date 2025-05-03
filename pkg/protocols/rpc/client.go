// pkg/protocols/rpc/client.go
package rpc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"time"
)

// RPCClient represents a client for sending RPC requests.
type RPCClient struct {
	pool *ConnectionPool
}

// NewRPCClient creates a new RPCClient with the given connection pool.
func NewRPCClient(pool *ConnectionPool) *RPCClient {
	return &RPCClient{
		pool: pool,
	}
}

// Call sends an RPC request and returns the response.
func (c *RPCClient) Call(ctx context.Context, req Request) (Response, error) {
	// Get a connection from the pool
	conn, err := c.pool.Get()
	if err != nil {
		return Response{}, err
	}
	defer c.pool.Put(conn)

	// Marshal the request
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}

	// Send the request over the connection
	err = sendRequest(conn, reqBytes)
	if err != nil {
		return Response{}, err
	}

	// Set a read deadline based on context
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetReadDeadline(deadline)
	} else {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	}

	// Read the response
	respBytes, err := readResponse(conn)
	if err != nil {
		return Response{}, err
	}

	// Unmarshal the response
	var resp Response
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		return Response{}, err
	}

	return resp, nil
}

// Helper functions for sending and receiving data over the connection

func sendRequest(conn net.Conn, data []byte) error {
	// Prefix data with length
	length := uint32(len(data))
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, length)
	message := append(lengthBuf, data...)

	_, err := conn.Write(message)
	return err
}

func readResponse(conn net.Conn) ([]byte, error) {
	// Read the length
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(conn, lengthBuf)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf)

	// Read the data
	data := make([]byte, length)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}
