package common

import (
	"fmt"
	"os"
	"os/user"
	"slices"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	log "github.com/sirupsen/logrus"
)

const CLAB_AUTHORISED_GROUP = "clab_admins"

func CheckAndGetRootPrivs(_ *cobra.Command, _ []string) error {
	_, euid, suid := unix.Getresuid()
	if euid != 0 && suid != 0 {
		return fmt.Errorf("this containerlab command requires root privileges or root via SUID to run, effective UID: %v SUID: %v", euid, suid)
	}

	if euid != 0 && suid == 0 {
		clabGroupExists := true
		clabGroup, err := user.LookupGroup(CLAB_AUTHORISED_GROUP)
		if err != nil {
			if _, ok := err.(user.UnknownGroupError); ok {
				log.Debug("Containerlab admin group does not exist, skipping group membership check")
				clabGroupExists = false
			} else {
				return fmt.Errorf("failed to lookup containerlab admin group: %v", err)
			}
		}

		if clabGroupExists {
			currentEffUser, err := user.Current()
			if err != nil {
				return err
			}

			effUserGroupIDs, err := currentEffUser.GroupIds()
			if err != nil {
				return err
			}

			if !slices.Contains(effUserGroupIDs, clabGroup.Gid) {
				return fmt.Errorf("user '%v' is not part of containerlab admin group 'clab_admins' (GID %v), which is required to execute this command.\nTo add yourself to this group, run the following command:\n\t$ sudo gpasswd -a %v clab_admins",
					currentEffUser.Username, clabGroup.Gid, currentEffUser.Username)
			}

			log.Debug("Group membership check passed")
		}

		err = obtainRootPrivs()
		if err != nil {
			return err
		}
	}

	return nil
}

func obtainRootPrivs() error {
	// Escalate to root privileges, changing saved UIDs to root/current group to be able to retain privilege escalation
	err := changePrivileges(0, os.Getgid(), 0, os.Getgid())
	if err != nil {
		return err
	}

	log.Debug("Obtained root privileges")

	return nil
}

func DropRootPrivs() error {
	// Drop privileges to the running user, retaining current saved IDs
	err := changePrivileges(os.Getuid(), os.Getgid(), -1, -1)
	if err != nil {
		return err
	}

	log.Debug("Dropped root privileges")

	return nil
}

func changePrivileges(new_uid, new_gid, saved_uid, saved_gid int) error {
	if err := unix.Setresuid(-1, new_uid, saved_uid); err != nil {
		return fmt.Errorf("failed to set UID: %v", err)
	}
	if err := unix.Setresgid(-1, new_gid, saved_gid); err != nil {
		return fmt.Errorf("failed to set GID: %v", err)
	}
	log.Debugf("Changed running UIDs to UID: %d GID: %d", new_uid, new_gid)
	return nil
}
