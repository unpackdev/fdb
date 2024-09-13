package fdb

type DbType string

func (t DbType) String() string {
	return string(t)
}

// HandlerType ...
type HandlerType int32

// Define the handlers
const (
	WriteHandlerType HandlerType = iota
	ReadHandlerType
)
