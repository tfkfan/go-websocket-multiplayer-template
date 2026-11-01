package entity

type SnapshotType uint8

const (
	SystemType SnapshotType = iota
	PrivateType
	BroadcastType
	CustomType
	TypeCount
)
