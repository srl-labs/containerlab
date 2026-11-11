// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabutils "github.com/srl-labs/containerlab/utils"
	"golang.org/x/sys/unix"
)

const (
	reasonNeedsRoot = "root privileges required: run with sudo or use a setuid " +
		"containerlab binary"
	reasonReadOnly = "disabled in read-only mode"
)

// Names of the capabilities, as used for the reasons they are unavailable.
const (
	capLifecycle = "lifecycle"
	capNetem     = "netem"
	capCapture   = "capture"
	capShare     = "share"
	capLinkState = "linkstate"
	capWireshark = "wireshark"
)

// privilege tells whether the commands that need root privileges can be run.
type privilege struct {
	ok bool
	// reason explains why they cannot.
	reason string
	// setuid is set when the containerlab binary escalates on its own, for every user who
	// may use it.
	setuid bool
}

// adminGroupCheck reports whether the group that guards the setuid containerlab binary
// exists and whether the user is a member of it.
type adminGroupCheck func() (exists, member bool, user string)

// detectPrivilege decides whether child processes obtain root privileges. They do when
// this process runs as root, as children inherit that, or when the containerlab binary is
// setuid root, in which case every child escalates on its own. A saved user ID of root
// shows that the running binary is setuid; the process itself keeps running as the user.
func detectPrivilege(euid, suid int, setuidBinary bool, check adminGroupCheck) privilege {
	if euid == 0 {
		return privilege{ok: true}
	}

	if suid != 0 && !setuidBinary {
		return privilege{reason: reasonNeedsRoot}
	}

	// A setuid containerlab only escalates for the members of its admin group, when that
	// group exists.
	if exists, member, name := check(); exists && !member {
		return privilege{reason: fmt.Sprintf(
			"user %q is not a member of the %s group, which the setuid containerlab binary "+
				"requires", name, clabutils.CLAB_AUTHORIZED_GROUP)}
	}

	return privilege{ok: true}
}

// currentPrivilege detects the privileges of this process with exe as the binary that is
// run for child commands.
func currentPrivilege(exe string) privilege {
	_, euid, suid := unix.Getresuid()

	priv := detectPrivilege(euid, suid, isSetuidRoot(exe), adminGroupMembership)
	priv.setuid = isSetuidRoot(exe)

	return priv
}

// isSetuidRoot reports whether running the file gives the process root privileges.
func isSetuidRoot(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.Mode()&os.ModeSetuid == 0 {
		return false
	}

	stat, ok := info.Sys().(*syscall.Stat_t)

	return ok && stat.Uid == 0
}

// adminGroupMembership looks the admin group up the way containerlab does. A failed lookup
// counts as a membership, as the command that is run then reports the precise problem.
func adminGroupMembership() (exists, member bool, name string) {
	exists, err := clabutils.UnixGroupExists(clabutils.CLAB_AUTHORIZED_GROUP)
	if err != nil || !exists {
		return false, false, ""
	}

	current, err := user.Current()
	if err != nil {
		return true, true, ""
	}

	member, err = clabutils.UserInUnixGroup(current.Username, clabutils.CLAB_AUTHORIZED_GROUP)
	if err != nil {
		return true, true, current.Username
	}

	return true, member, current.Username
}

// capabilities derives what the UI can offer from the privileges and the environment.
func (b *Backend) capabilities() clabinternaltuibackend.Capabilities {
	caps := clabinternaltuibackend.Capabilities{Reasons: map[string]string{}}

	switch {
	case b.opts.ReadOnly:
		for _, name := range []string{capLifecycle, capNetem, capCapture, capShare} {
			caps.Reasons[name] = reasonReadOnly
		}
	case !b.priv.ok:
		for _, name := range []string{capLifecycle, capNetem, capCapture, capShare} {
			caps.Reasons[name] = b.priv.reason
		}
	default:
		caps.Lifecycle, caps.Netem, caps.Capture, caps.Share = true, true, true, true
	}

	// The state of the interfaces becomes known once its stream runs.
	if b.priv.ok {
		caps.Reasons[capLinkState] = "the interface state stream is not running yet"
	} else {
		caps.Reasons[capLinkState] = b.priv.reason
	}

	if reason := b.wiresharkUnavailable(); reason != "" {
		caps.Reasons[capWireshark] = reason
	} else {
		caps.Wireshark = true
	}

	return caps
}

// wiresharkUnavailable returns why Wireshark cannot be started, empty when it can.
func (b *Backend) wiresharkUnavailable() string {
	if _, err := b.lookPath(wiresharkBinary); err != nil {
		return "wireshark is not installed"
	}

	if b.getenv("DISPLAY") == "" && b.getenv("WAYLAND_DISPLAY") == "" {
		return "no graphical display: neither DISPLAY nor WAYLAND_DISPLAY is set"
	}

	return ""
}

// sudoUser describes the user who invoked sudo.
type sudoUser struct {
	cred *syscall.Credential
	// env holds the variables that differ for that user.
	env []string
}

// invokingUser returns the user a program should run as when this process only is root
// because of sudo: a desktop program belongs to the session of the user who typed the
// command. It returns nil when there is no such user.
func invokingUser(euid int, getenv func(string) string) *sudoUser {
	if euid != 0 {
		return nil
	}

	uid, err := strconv.ParseUint(getenv("SUDO_UID"), 10, 32)
	if err != nil || uid == 0 {
		return nil
	}

	gid, err := strconv.ParseUint(getenv("SUDO_GID"), 10, 32)
	if err != nil {
		gid = uid
	}

	who := &sudoUser{cred: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}

	account, err := user.LookupId(strconv.FormatUint(uid, 10))
	if err != nil {
		return who
	}

	who.env = []string{
		"HOME=" + account.HomeDir,
		"USER=" + account.Username,
		"LOGNAME=" + account.Username,
	}

	// The supplementary groups of root must not carry over to the user's program.
	if ids, err := account.GroupIds(); err == nil {
		for _, id := range ids {
			if n, err := strconv.ParseUint(id, 10, 32); err == nil {
				who.cred.Groups = append(who.cred.Groups, uint32(n))
			}
		}
	}

	// A Wayland session is found through the runtime directory, which sudo does not pass on.
	runtimeDir := fmt.Sprintf("/run/user/%d", uid)
	if _, err := os.Stat(runtimeDir); err == nil && getenv("XDG_RUNTIME_DIR") == "" {
		who.env = append(who.env, "XDG_RUNTIME_DIR="+runtimeDir)
	}

	return who
}

// withEnv returns environ with the given entries replacing the variables of the same name.
func withEnv(environ, entries []string) []string {
	out := make([]string, 0, len(environ)+len(entries))

	for _, entry := range environ {
		replaced := false

		for _, repl := range entries {
			if envName(repl) == envName(entry) {
				replaced = true

				break
			}
		}

		if !replaced {
			out = append(out, entry)
		}
	}

	return append(out, entries...)
}
