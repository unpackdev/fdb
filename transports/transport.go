package transports

import "context"

type Transport interface {
	Addr() string
	Start(ctx context.Context) error
	Stop() error
}
