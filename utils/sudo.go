package utils

import (
	"fmt"
	"os"
	"os/user"

	"golang.org/x/sys/unix"

	"github.com/charmbracelet/log"
)

const (
	CLAB_AUTHORIZED_GROUP = "clab_admins"
	ROOT_UID              = 0
	NOMODIFY              = -1
)

func CheckAndGetRootPrivs() error {
	_, euid, suid := unix.Getresuid()
	if euid != 0 && suid != 0 {
		return fmt.Errorf("this containerlab command requires root privileges or root via SUID to run, effective UID: %v SUID: %v", euid, suid)
	}

	// If we are not running directly as root, and SUID is properly set, attempt to get root privileges
	if euid != 0 && suid == 0 {
		clabGroupExists, err := UnixGroupExists(CLAB_AUTHORIZED_GROUP)
		if err != nil {
			return fmt.Errorf("failed to lookup containerlab admin group: %w", err)
		}

		if clabGroupExists {
			currentEffUser, err := user.Current()
			if err != nil {
				return fmt.Errorf("failed to retrieve current user details: %w", err)
			}

			userInClabGroup, err := UserInUnixGroup(currentEffUser.Username, CLAB_AUTHORIZED_GROUP)
			if err != nil {
				return fmt.Errorf("failed to check containerlab admin group membership: %w", err)
			}

			if !userInClabGroup {
				return fmt.Errorf("user '%v' is not part of containerlab admin group 'clab_admins', which is required to execute this command.\nTo add yourself to this group, run the following command:\n\t$ sudo usermod -aG clab_admins %v",
					currentEffUser.Username, currentEffUser.Username)
			}

			log.Debug("Group membership check passed")
		} else {
			log.Debug("Containerlab admin group 'clab_admins' does not exist, skipping group membership check")
		}

		err = obtainRootPrivs()
		if err != nil {
			return fmt.Errorf("failed to obtain root privileges: %w", err)
		}
	} else if euid == 0 {
		log.Debugf("Already running as root, skipping root privilege escalation")
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
		return fmt.Errorf("failed to set UID: %w", err)
	}
	if err := unix.Setresgid(-1, new_gid, saved_gid); err != nil {
		return fmt.Errorf("failed to set GID: %w", err)
	}
	log.Debugf("Changed running UIDs to UID: %d GID: %d", new_uid, new_gid)
	return nil
}
