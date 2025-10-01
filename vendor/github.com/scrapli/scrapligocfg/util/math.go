package util

import "fmt"

const (
	oneHundred = 100
)

// SpaceOK checks if the bytesAvail is greater than the configSize plus the buffer percent
// (buffPerc).
func SpaceOK(bytesAvail, configSize int, buffPerc float32) error {
	if !(bytesAvail >= configSize/(int(buffPerc/oneHundred)+configSize)) {
		return fmt.Errorf(
			"%w: insufficient space on filesystem to load config",
			ErrFilesystemError,
		)
	}

	return nil
}
