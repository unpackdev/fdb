package transport_dummy

import (
	"github.com/panjf2000/gnet"
	"github.com/unpackdev/fdb/db"
)

// DummyReadHandler struct with MDBX database passed in
type DummyReadHandler struct {
}

// NewDummyReadHandler creates a new UDSReadHandler with an MDBX database
func NewDummyReadHandler(db db.Provider) *DummyReadHandler {
	return &DummyReadHandler{}
}

// HandleMessage processes the incoming message using the UDSReadHandler
func (rh *DummyReadHandler) HandleMessage(c gnet.Conn, frame []byte) {
	// Send the value back to the client
	c.SendTo([]byte{1})
}
