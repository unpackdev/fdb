package types

// Target specifies the distribution target type
type Target uint8

const (
	TargetAll Target = iota
	TargetValidators
	TargetDirectPeer
)
