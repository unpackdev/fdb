package client

type MessageType byte

func (t MessageType) Uint64() uint64 {
	return uint64(t)
}

var (
	InvalidActionMessageType MessageType = 0x69
	WriteSuccessMessageType  MessageType = 0x00
)
