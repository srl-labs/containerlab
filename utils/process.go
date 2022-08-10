package utils

import "syscall"

// PauseProcessGroup sends the SIGSTOP signal to a process group, causing all
// the processes within the group to be Paused e.g. SRL runs multilpe processes, if the
// container is meant to be stopped, all the related processes must be paused.
// To me it seams like the ProcessGroupID is set correctly so we can count on that field.
// The syscall.Kill interpretes negative ints as a PGID and not as a common PID.
func PauseProcessGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGSTOP)
}

// UnpauseProcessGroup send the SIGCONT to the given ProcessGroup identified by its ID.
func UnpauseProcessGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGCONT)
}
