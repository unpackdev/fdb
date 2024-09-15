package pprof

import (
	"context"
	"github.com/unpackdev/unpack/pkg/options"
	"go.uber.org/zap"
	"net/http"
	_ "net/http/pprof"
)

// Pprof encapsulates the pprof server configuration.
type Pprof struct {
	ctx  context.Context
	opts options.Pprof
}

// New creates a new Pprof instance with the specified listen address.
func New(ctx context.Context, opts options.Pprof) *Pprof {
	return &Pprof{ctx: ctx, opts: opts}
}

// IsEnabled returns if pprof server is enabled or not (via configuration)
func (p *Pprof) IsEnabled() bool {
	return p.opts.Enabled
}

// GetName returns options service name associated with pprof server
func (p *Pprof) GetName() string {
	return p.opts.Name
}

// GetAddress returns the address on which pprof server will be listening on
func (p *Pprof) GetAddress() string {
	return p.opts.Addr
}

// Start initializes the pprof HTTP server on the configured address.
func (p *Pprof) Start() error {
	zap.L().Info(
		"Started up pprof server",
		zap.String("service", p.opts.Name),
		zap.String("address", p.opts.Addr),
	)
	return http.ListenAndServe(p.opts.Addr, nil)
}
