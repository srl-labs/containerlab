package types

// SaveConfigResult captures artifacts produced by a SaveConfig operation.
// Either FilePaths or Payload should be set.
type SaveConfigResult struct {
	FilePaths   []string
	Payload     []byte
	PayloadName string
}
