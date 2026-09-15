package iptables

import (
	"testing"
)

func TestIptablesSysProcAttr(t *testing.T) {
	t.Run("setuid parent needs matching root uids in child", func(t *testing.T) {
		attr := iptablesSysProcAttr(1000, 0, 1000)
		if attr == nil || attr.Credential == nil {
			t.Fatal("expected Credential for setuid-root parent")
		}
		if attr.Credential.Uid != 0 {
			t.Fatalf("Uid = %d, want 0", attr.Credential.Uid)
		}
		if attr.Credential.Gid != 1000 {
			t.Fatalf("Gid = %d, want 1000 (preserve egid)", attr.Credential.Gid)
		}
		if !attr.Credential.NoSetGroups {
			t.Fatal("expected NoSetGroups to preserve supplementary groups")
		}
	})

	t.Run("fully root parent needs no credential", func(t *testing.T) {
		if attr := iptablesSysProcAttr(0, 0, 0); attr != nil {
			t.Fatalf("expected nil SysProcAttr, got %#v", attr)
		}
	})

	t.Run("non-root parent needs no credential", func(t *testing.T) {
		if attr := iptablesSysProcAttr(1000, 1000, 1000); attr != nil {
			t.Fatalf("expected nil SysProcAttr, got %#v", attr)
		}
	})
}

func TestNewIptablesCmdClearsEnv(t *testing.T) {
	cmd := newIptablesCmd("iptables", "-V")
	if cmd.Env == nil {
		t.Fatal("Env is nil (would inherit parent); want empty slice")
	}
	if len(cmd.Env) != 0 {
		t.Fatalf("Env = %#v, want empty", cmd.Env)
	}
}
