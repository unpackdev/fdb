package accounts

import "github.com/unpackdev/fdb/types"

// TODO: Not even sure how to do this... And is this even how it should be done...
// Perhaps based on node type and configuration instead of anything else...
func (a *Account) SupportedTransports() []types.TransportType {
	return []types.TransportType{}
}
