package types

type HandlerStatus byte

const (
	HandlerStatusError   HandlerStatus = 0x00
	HandlerStatusSuccess HandlerStatus = 0x01
)

func (hs HandlerStatus) Byte() byte {
	return byte(hs)
}

func (hs HandlerStatus) String() string {
	switch hs {
	case HandlerStatusSuccess:
		return "success"
	case HandlerStatusError:
		return "error"
	default:
		return "unknown"
	}
}
