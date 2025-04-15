// utils/users.go
package utils

import (
	"os/user"

	"github.com/charmbracelet/log"
)

// IsInClabAdminsGroup checks if the current user is in the clab_admins group
func IsInClabAdminsGroup() bool {
	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		log.Debug("Failed to get current user", "error", err)
		return false
	}

	// Get groups the user belongs to
	groupIds, err := currentUser.GroupIds()
	if err != nil {
		log.Debug("Failed to get user groups", "error", err)
		return false
	}

	// Check if clab_admins group exists
	clabAdminsGroup, err := user.LookupGroup("clab_admins")
	if err != nil {
		log.Debug("clab_admins group not found", "error", err)
		return false
	}

	// Check if user is in clab_admins group
	for _, gid := range groupIds {
		if gid == clabAdminsGroup.Gid {
			return true
		}
	}

	return false
}
