package fdb

import (
	"github.com/panjf2000/gnet"
	"github.com/unpackdev/fdb/db"
)

// DummyWriteHandler struct with MDBX database passed in
type DummyWriteHandler struct {
	db db.Provider // MDBX database instance
}

// NewDummyWriteHandler creates a new UDSWriteHandler with an MDBX database
func NewDummyWriteHandler(db db.Provider) *DummyWriteHandler {
	return &DummyWriteHandler{}
}

// HandleMessage processes the incoming message using the UDSWriteHandler
func (wh *DummyWriteHandler) HandleMessage(c gnet.Conn, frame []byte) {
	// Send success response
	c.SendTo([]byte{1})
}
