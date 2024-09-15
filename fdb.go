package fdb

import (
	"context"
	"github.com/pkg/errors"
	"github.com/unpackdev/fdb/pkg/config"
)

type FDB struct {
	ctx    context.Context
	config config.Config
}

func New(ctx context.Context, config config.Config) (*FDB, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "failure to validate (f)db configuration")
	}

	return &FDB{
		ctx:    ctx,
		config: config,
	}, nil
}
