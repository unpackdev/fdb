package fdb

import (
	"context"
	"fmt"
	"github.com/unpackdev/fdb/pkg/config"
)

type Manager struct {
	ctx  context.Context
	opts config.MdbxNodes
	dbs  map[DbType]Provider
}

func NewManager(ctx context.Context, opts config.MdbxNodes) (*Manager, error) {
	dbs := make(map[DbType]Provider)
	for _, node := range opts {
		db, err := NewDb(ctx, node)
		if err != nil {
			return nil, err
		}
		dbs[DbType(node.Name)] = db
	}
	return &Manager{ctx: ctx, opts: opts, dbs: dbs}, nil
}

func (m *Manager) GetDb(name DbType) (Provider, error) {
	db, ok := m.dbs[name]
	if !ok {
		return nil, fmt.Errorf("mdbx database not found: %s", name)
	}
	return db, nil
}

func (m *Manager) Close() error {
	for _, db := range m.dbs {
		if err := db.Close(); err != nil {
			return err
		}
	}
	return nil
}
