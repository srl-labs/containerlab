package iptables

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// newIptablesCmd builds an iptables/ip6tables command that is safe to run from a
// setuid-root containerlab binary (ruid=user, euid=0).
//
// iptables ≥ 1.8.8 exits 111 when getuid() != geteuid() because it loads match/
// target shared libraries and therefore refuses to run under a setuid parent.
// We isolate the fix to the child process via Credential so the long-lived
// containerlab process keeps its real UID (important for $HOME file ownership).
// LD_* is stripped so a poisoned library path cannot follow into iptables.
func newIptablesCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = sanitizeIptablesEnv(os.Environ())

	if attr := iptablesSysProcAttr(os.Getuid(), os.Geteuid(), os.Getegid()); attr != nil {
		cmd.SysProcAttr = attr
	}

	return cmd
}

// iptablesSysProcAttr returns SysProcAttr that sets the child's real/effective
// UIDs to root when the parent is running setuid-root. Nil when no change is needed.
func iptablesSysProcAttr(ruid, euid, egid int) *syscall.SysProcAttr {
	if euid != 0 || ruid == 0 {
		return nil
	}

	return &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         0,
			Gid:         uint32(egid),
			NoSetGroups: true,
		},
	}
}

// sanitizeIptablesEnv drops LD_* (and similar) loader overrides from env.
func sanitizeIptablesEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if isUnsafeIptablesEnvKey(key) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func isUnsafeIptablesEnvKey(key string) bool {
	// iptables loads xtables plugins via dlopen; LD_* would defeat the setuid check's intent.
	return strings.HasPrefix(key, "LD_")
}
