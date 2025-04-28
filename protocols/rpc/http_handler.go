// pkg/protocols/rpc/http_handler.go
package rpc

import (
	"bufio"
	"bytes"
	"context"
	"github.com/unpackdev/fdb/logger"
	"github.com/unpackdev/fdb/transports/tcp"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/panjf2000/gnet/v2"
	"go.uber.org/zap"
)

// Define a struct to hold the HTTP request and a mutable header map for responses
type httpContext struct {
	Request         *http.Request
	ResponseHeaders http.Header
}

const httpContextKey contextKey = "httpContext"

// HTTPHandler handles RPC over HTTP requests.
type HTTPHandler struct {
	server *Server
	logger logger.Logger
}

var (
	// Buffer pool for response buffers
	responseBufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

// NewHTTPHandler creates a new HTTPHandler.
func NewHTTPHandler(server *Server, logger logger.Logger) *HTTPHandler {
	return &HTTPHandler{
		server: server,
		logger: logger,
	}
}

// Handle processes incoming data and sends responses.
func (h *HTTPHandler) Handle(ctx *tcp.ConnectionContext, conn gnet.Conn) gnet.Action {
	data, err := conn.Next(-1)
	if err != nil {
		h.logger.Error("Failed to read data", zap.Error(err))
		return gnet.Close
	}

	// Append data to the buffer
	ctx.Buffer = append(ctx.Buffer, data...)
	h.logger.Debug("New http handler request received", zap.Int("bytes", len(data)))

	// Initialize readers if nil
	if ctx.BytesReader == nil {
		ctx.BytesReader = bytes.NewReader(ctx.Buffer)
		ctx.BufioReader = bufio.NewReader(ctx.BytesReader)
	} else {
		ctx.BytesReader.Reset(ctx.Buffer)
		ctx.BufioReader.Reset(ctx.BytesReader)
	}

	for {
		// Save the current state of the reader
		markReader := *ctx.BufioReader

		// Use http.ReadRequest to parse the request
		req, err := http.ReadRequest(ctx.BufioReader)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				h.logger.Debug("Incomplete HTTP request received; waiting for more data")
				break
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
			response.Header.Set("Connection", "close")
			h.sendResponse(conn, response)
			// Clear the buffer since the request is malformed
			ctx.Buffer = ctx.Buffer[:0]
			ctx.BytesReader.Reset(ctx.Buffer)
			ctx.BufioReader.Reset(ctx.BytesReader)
			return gnet.None
		}

		// Check if the entire body has been read
		if req.ContentLength > 0 {
			body := make([]byte, req.ContentLength)
			_, err := io.ReadFull(ctx.BufioReader, body)
			if err != nil {
				if err == io.ErrUnexpectedEOF || err == io.EOF {
					h.logger.Debug("Incomplete HTTP request body received; waiting for more data")
					// Restore the reader to the previous state
					*ctx.BufioReader = markReader
					break
				}
				h.logger.Warn("Error reading request body", zap.Error(err))
				response := &http.Response{
					StatusCode: http.StatusBadRequest,
					Status:     http.StatusText(http.StatusBadRequest),
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					Header:     make(http.Header),
					Body:       http.NoBody,
				}
				response.Header.Set("Connection", "close")
				h.sendResponse(conn, response)
				// Clear the buffer since the request is malformed
				ctx.Buffer = ctx.Buffer[:0]
				ctx.BytesReader.Reset(ctx.Buffer)
				ctx.BufioReader.Reset(ctx.BytesReader)
				return gnet.None
			}
			req.Body = io.NopCloser(bytes.NewReader(body))
		}

		// Calculate the number of bytes read
		reqLen := len(ctx.Buffer) - ctx.BufioReader.Buffered()

		// Handle the request
		response := h.handleHTTPRequest(req)

		// Send the response
		h.sendResponse(conn, response)

		// Check if we should close the connection
		if shouldClose(req, response) {
			return gnet.Close
		}

		// Remove the processed request from the buffer
		ctx.Buffer = ctx.Buffer[reqLen:]
		ctx.BytesReader.Reset(ctx.Buffer)
		ctx.BufioReader.Reset(ctx.BytesReader)

		// If no more data, break the loop
		if ctx.BufioReader.Buffered() == 0 {
			break
		}
	}

	return gnet.None
}

// Helper function to determine if the connection should be closed
func shouldClose(req *http.Request, resp *http.Response) bool {
	if req.Close || resp.Close {
		return true
	}
	if req.ProtoMajor < 1 || (req.ProtoMajor == 1 && req.ProtoMinor == 0) {
		// HTTP/1.0 defaults to close unless Connection: keep-alive
		return !strings.EqualFold(req.Header.Get("Connection"), "keep-alive")
	}
	// For HTTP/1.1, default is keep-alive unless Connection: close
	return strings.EqualFold(req.Header.Get("Connection"), "close")
}

func (h *HTTPHandler) sendResponse(conn gnet.Conn, resp *http.Response) {
	// Get a buffer from the pool
	buffer := responseBufferPool.Get().(*bytes.Buffer)
	buffer.Reset()                       // Ensure the buffer is empty
	defer responseBufferPool.Put(buffer) // Return the buffer to the pool

	// Write the response to the buffer
	err := resp.Write(buffer)
	if err != nil {
		h.logger.Error("Failed to write response", zap.Error(err))
		conn.Close()
		return
	}

	// Asynchronously write the response buffer to the connection
	conn.AsyncWrite(buffer.Bytes(), nil)
}

func (h *HTTPHandler) handleHTTPRequest(req *http.Request) *http.Response {
	// Handle CORS preflight requests
	if req.Method == "OPTIONS" {
		return h.handleCORSPreflight(req)
	}

	switch {
	case req.Method == "POST" && req.URL.Path == "/rpc":
		return h.handleRPCEndpoint(req)
	default:
		// Return 404 Not Found
		resp := &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     http.StatusText(http.StatusNotFound),
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    req,
		}
		return resp
	}
}

func (h *HTTPHandler) handleCORSPreflight(req *http.Request) *http.Response {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}

	h.setCORSHeaders(resp.Header)

	// Allow the methods that your server supports
	resp.Header.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	// Allow the headers that your server expects
	resp.Header.Set("Access-Control-Allow-Headers", "Content-Type")
	// You might want to set Access-Control-Max-Age
	resp.Header.Set("Access-Control-Max-Age", "86400") // Cache preflight response for 1 day

	// Close the connection after preflight response
	resp.Header.Set("Connection", "close")
	resp.Close = true

	return resp
}

func (h *HTTPHandler) setCORSHeaders(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	//header.Set("Access-Control-Allow-Headers", "Content-Type")

	// If you need to restrict origins, replace "*" with the allowed origin(s)

	// If you expect specific headers from the client, specify them
	// header.Set("Access-Control-Allow-Headers", "Content-Type")

	// Optionally, expose certain headers to the client
	// header.Set("Access-Control-Expose-Headers", "Content-Length")
}

func (h *HTTPHandler) handleRPCEndpoint(req *http.Request) *http.Response {
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", zap.Error(err))
		return h.buildErrorResponse(http.StatusInternalServerError, req, true)
	}

	// Log the received JSON data
	h.logger.Debug("Received RPC JSON data", zap.ByteString("body", bodyBytes))

	// Create an httpContext and embed it into a new context
	hc := &httpContext{
		Request:         req,
		ResponseHeaders: make(http.Header),
	}
	ctx := context.WithValue(context.Background(), httpContextKey, hc)

	// Process the RPC request through the server (which applies middleware)
	respBytes, err := h.server.HandleRawRequest(ctx, bodyBytes)
	if err != nil {
		h.logger.Error("RPC processing failed", zap.Error(err))
		return h.buildErrorResponse(http.StatusInternalServerError, req, true)
	}

	// Log the response JSON data
	h.logger.Debug("Sending RPC JSON response", zap.ByteString("response", respBytes))

	// Build HTTP response with application/json content type
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Status:        http.StatusText(http.StatusOK),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		ContentLength: int64(len(respBytes)),
		Body:          io.NopCloser(bytes.NewReader(respBytes)),
		Request:       req,
	}

	// Set Content-Type
	resp.Header.Set("Content-Type", "application/json")

	// Apply headers set by middleware
	for key, values := range hc.ResponseHeaders {
		for _, value := range values {
			resp.Header.Add(key, value)
		}
	}

	// Determine if we should close the connection
	if shouldClose(req, resp) {
		resp.Header.Set("Connection", "close")
		resp.Close = true
	} else {
		resp.Header.Set("Connection", "keep-alive")
		resp.Close = false
	}

	h.setCORSHeaders(resp.Header)

	return resp
}

// Helper method to build error responses
func (h *HTTPHandler) buildErrorResponse(statusCode int, req *http.Request, close bool) *http.Response {
	resp := &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
		Close:      close,
	}
	resp.Header.Set("Connection", "close")
	// Set CORS headers
	h.setCORSHeaders(resp.Header)
	return resp
}

// OnClose is called when the connection is closed.
func (h *HTTPHandler) OnClose(ctx *tcp.ConnectionContext, conn gnet.Conn) {
	h.logger.Debug("HTTP connection closed", zap.String("remote_addr", conn.RemoteAddr().String()))
}
