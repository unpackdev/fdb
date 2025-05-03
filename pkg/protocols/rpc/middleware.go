// pkg/protocols/rpc/middleware.go
package rpc

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/pkg/logger"

	"github.com/goccy/go-json"
)

// Middleware defines a function to process middleware.
type Middleware func(HandlerFunc) HandlerFunc

// ChainMiddleware applies middlewares to a handler.
func ChainMiddleware(handler HandlerFunc, middlewares ...Middleware) HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// LoggingMiddleware logs the start and end of a request.
func LoggingMiddleware(logger logger.Logger) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
			logger.Info("Handling RPC request")
			result, err := next(ctx, params)
			logger.Info("Finished handling RPC request")
			return result, err
		}
	}
}

// CORSMiddleware handles CORS for HTTP transport.
func CORSMiddleware(allowedOrigins []string, allowedMethods []string, allowedHeaders []string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
			transportType, _ := ctx.Value("transport").(string)

			if transportType == "http" {
				req, respHeaders, err := getHTTPResponseRequest(ctx)
				if err != nil {
					return nil, NewError(InternalError, "Failed to retrieve HTTP context")
				}

				origin := req.Header.Get("Origin")
				if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
					respHeaders.Set("Access-Control-Allow-Origin", origin)
					respHeaders.Set("Vary", "Origin")
					respHeaders.Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
					respHeaders.Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
					respHeaders.Set("Access-Control-Allow-Credentials", "true")
				}

				// Handle preflight OPTIONS request
				if req.Method == http.MethodOptions {
					// Indicate that no further processing is needed
					return nil, nil
				}
			}

			return next(ctx, params)
		}
	}
}

// BasicAuthMiddleware handles HTTP Basic Authentication.
func BasicAuthMiddleware(validUsers map[string]string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, params json.RawMessage) (interface{}, *Error) {
			transportType, _ := ctx.Value("transport").(string)

			if transportType == "http" {
				req, respHeaders, err := getHTTPResponseRequest(ctx)
				if err != nil {
					return nil, NewError(InternalError, "Failed to retrieve HTTP context")
				}

				authHeader := req.Header.Get("Authorization")
				if authHeader == "" {
					// Missing Authorization header
					respHeaders.Set("WWW-Authenticate", `Basic realm="Restricted"`)
					return nil, NewError(Unauthorized, "Authorization header required")
				}

				// Check if the Authorization header is of type Basic
				if !strings.HasPrefix(authHeader, "Basic ") {
					// Unsupported authentication scheme
					respHeaders.Set("WWW-Authenticate", `Basic realm="Restricted"`)
					return nil, NewError(Unauthorized, "Unsupported authentication scheme")
				}

				// Decode the Base64 encoded credentials
				encodedCreds := strings.TrimPrefix(authHeader, "Basic ")
				decodedCreds, err := base64.StdEncoding.DecodeString(encodedCreds)
				if err != nil {
					// Invalid Base64 encoding
					respHeaders.Set("WWW-Authenticate", `Basic realm="Restricted"`)
					return nil, NewError(Unauthorized, "Invalid authentication credentials")
				}

				// Split the credentials into username and password
				creds := strings.SplitN(string(decodedCreds), ":", 2)
				if len(creds) != 2 {
					// Malformed credentials
					respHeaders.Set("WWW-Authenticate", `Basic realm="Restricted"`)
					return nil, NewError(Unauthorized, "Malformed authentication credentials")
				}

				username, password := creds[0], creds[1]

				// Validate the credentials
				expectedPassword, userExists := validUsers[username]
				if !userExists || expectedPassword != password {
					// Invalid username or password
					respHeaders.Set("WWW-Authenticate", `Basic realm="Restricted"`)
					return nil, NewError(Unauthorized, "Invalid username or password")
				}
			}

			// Proceed to the next middleware or handler
			return next(ctx, params)
		}
	}
}

// getHTTPResponseRequest extracts the *http.Request and response headers from the context
func getHTTPResponseRequest(ctx context.Context) (*http.Request, http.Header, error) {
	hc, ok := ctx.Value(httpContextKey).(*httpContext)
	if !ok {
		return nil, nil, errors.New("no HTTP context found in context")
	}
	return hc.Request, hc.ResponseHeaders, nil
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, o := range allowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}
