package fdb

import (
	"context"
	"github.com/pkg/errors"
)

type FDB struct {
	ctx    context.Context
	config Config
}

func New(ctx context.Context, config Config) (*FDB, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "failure to validate (f)db configuration")
	}

	return &FDB{
		ctx:    ctx,
		config: config,
	}, nil
}
