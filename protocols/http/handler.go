// handler.go
package http

import (
	"bufio"
	"bytes"
	"github.com/goccy/go-json"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/transports/tcp"
	"io"
	"net/http"

	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// TrafficHandler processes raw HTTP data.
type TrafficHandler struct {
	logger logger.Logger
}

// NewHTTPTrafficHandler creates a new instance of TrafficHandler.
func NewHTTPTrafficHandler(logger logger.Logger) *TrafficHandler {
	return &TrafficHandler{
		logger: logger,
	}
}

// Handle processes incoming data and sends responses.
func (h *TrafficHandler) Handle(ctx *tcp.ConnectionContext, conn gnet.Conn) gnet.Action {
	data, err := conn.Next(-1)
	if err != nil {
		h.logger.Error("Failed to read data", zap.Error(err))
		return gnet.Close
	}

	// Append data to the buffer
	ctx.Buffer = append(ctx.Buffer, data...)
	h.logger.Debug("Data received", zap.Int("bytes", len(data)))

	// Create a new bufio.Reader from the buffer
	reader := bufio.NewReader(bytes.NewReader(ctx.Buffer))

	// Use http.ReadRequest to parse the request
	req, err := http.ReadRequest(reader)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			h.logger.Debug("Incomplete HTTP request received; waiting for more data")
			return gnet.None
		}
		h.logger.Warn("Malformed HTTP request", zap.Error(err))
		response := &http.Response{
			StatusCode: http.StatusBadRequest,
			Status:     http.StatusText(http.StatusBadRequest),
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Body:       http.NoBody,
		}
		// Set Connection: close
		response.Header.Set("Connection", "close")
		h.sendResponse(conn, response)
		// Clear the buffer since the request is malformed
		ctx.Buffer = ctx.Buffer[:0]
		return gnet.None
	}

	// Calculate the number of bytes read
	reqLen := len(ctx.Buffer) - reader.Buffered()

	// Clear the buffer up to the parsed request
	ctx.Buffer = ctx.Buffer[reqLen:]

	// Handle the request
	response := h.handleHTTPRequest(req)

	// Send the response
	h.sendResponse(conn, response)

	return gnet.None
}

// sendResponse writes the response to the connection
func (h *TrafficHandler) sendResponse(conn gnet.Conn, resp *http.Response) {
	// Indicate that the connection should be closed
	resp.Close = true

	// Create a buffer to hold the response
	var buffer bytes.Buffer
	// Write the response to the buffer
	err := resp.Write(&buffer)
	if err != nil {
		h.logger.Error("Failed to write response", zap.Error(err))
		conn.Close()
		return
	}
	// Asynchronously write the response buffer to the connection
	conn.AsyncWrite(buffer.Bytes(), func(c gnet.Conn, writeErr error) error {
		if writeErr != nil {
			h.logger.Error("Failed to write response", zap.Error(writeErr))
		}
		c.Close()
		return nil
	})
}

// handleHTTPRequest processes the parsed HTTP request and returns a response.
func (h *TrafficHandler) handleHTTPRequest(req *http.Request) *http.Response {
	// Prepare the response
	resp := &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Request:    req, // Set the Request field
	}
	resp.Close = true // Indicate that the connection should be closed

	switch {
	case req.Method == "GET" && req.URL.Path == "/":
		resp.StatusCode = http.StatusOK
		resp.Status = http.StatusText(http.StatusOK)
		resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
		resp.Header.Set("Connection", "close") // Explicitly set Connection: close
		body := []byte("Welcome to the HTTP Server!")
		resp.ContentLength = int64(len(body))
		resp.Body = io.NopCloser(bytes.NewReader(body))

	case req.Method == "POST" && req.URL.Path == "/echo":
		return h.handleEchoEndpoint(req)

	default:
		resp.StatusCode = http.StatusNotFound
		resp.Status = http.StatusText(http.StatusNotFound)
		resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
		resp.Header.Set("Connection", "close") // Explicitly set Connection: close
		body := []byte("Not Found")
		resp.ContentLength = int64(len(body))
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}

	return resp
}

// handleEchoEndpoint handles the /echo POST endpoint.
func (h *TrafficHandler) handleEchoEndpoint(req *http.Request) *http.Response {
	resp := &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Request:    req, // Set the Request field
	}
	resp.Close = true                      // Indicate that the connection should be closed
	resp.Header.Set("Connection", "close") // Explicitly set Connection: close

	var payload struct {
		Message string `json:"message"`
	}

	err := json.NewDecoder(req.Body).Decode(&payload)
	if err != nil {
		resp.StatusCode = http.StatusBadRequest
		resp.Status = http.StatusText(http.StatusBadRequest)
		resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
		body := []byte("Invalid JSON payload")
		resp.ContentLength = int64(len(body))
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp
	}

	resp.StatusCode = http.StatusOK
	resp.Status = http.StatusText(http.StatusOK)
	resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
	body := []byte(payload.Message)
	resp.ContentLength = int64(len(body))
	resp.Body = io.NopCloser(bytes.NewReader(body))

	return resp
}
