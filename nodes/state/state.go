package state

type NodeState uint

const (
	Unknown NodeState = iota
	// Deployed means the underlying container has been started and deploy function succeeded.
	Deployed
)
