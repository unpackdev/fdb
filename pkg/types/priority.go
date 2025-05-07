package types

// Priority represents the importance level of a record batch
type Priority uint8

const (
	PriorityHigh Priority = iota
	PriorityNormal
	PriorityLow
)
