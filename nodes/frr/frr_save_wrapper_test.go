// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package frr

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
	"golang.org/x/sys/unix"

	clabexec "github.com/srl-labs/containerlab/exec"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// These interposers execute the real filesystem operations. Each probe models
// FRR starting vtysh between installer steps, without sleeps or production hooks.
const saveWrapperInterposers = `
frr_probe() {
    local status=0
    "$FRR_TEST_VTYSH" -b 'two words' > "$FRR_TEST_OUTPUT" 2>&1 || status=$?
    builtin printf '%s:%s\n' "$1" "$status" >> "$FRR_TEST_PROBES"
}
ln() {
    [ "$FRR_TEST_FAULT" != link ] || return 91
    command ln "$@" || return $?
    frr_probe link
    [ "$FRR_TEST_FAULT" != link-interrupted ] || kill -KILL $$
}
mv() {
    [ "$FRR_TEST_FAULT" != publish ] || return 91
    command mv "$@" || return $?
    frr_probe publish
}
printf() {
    if [ "$FRR_TEST_FAULT" = write ]; then
        builtin printf 'partial wrapper'
        return 91
    fi
    builtin printf "$@"
}
chmod() {
    [ "$FRR_TEST_FAULT" != chmod ] || return 91
    command chmod "$@" || return $?
    frr_probe chmod
}
`

type saveWrapperFixture struct {
	binDir    string
	configDir string
	path      string
	original  []byte
}

func newSaveWrapperFixture(t *testing.T) saveWrapperFixture {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "with spaces")
	f := saveWrapperFixture{
		binDir:    filepath.Join(dir, "bin"),
		configDir: filepath.Join(dir, "config"),
		original:  []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"),
	}
	f.path = filepath.Join(f.binDir, "vtysh")
	for _, path := range []string{f.binDir, f.configDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(f.path, f.original, 0o751); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FRR_TEST_VTYSH", f.path)
	t.Setenv("FRR_TEST_PROBES", filepath.Join(dir, "probes"))
	t.Setenv("FRR_TEST_OUTPUT", filepath.Join(dir, "output"))
	t.Setenv("FRR_TEST_FAULT", "")

	return f
}

func (f saveWrapperFixture) install(t *testing.T) error {
	t.Helper()

	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n.WithRuntime(rt)
	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			args := append([]string(nil), cmd.GetCmd()...)
			for _, i := range []int{2, 4} {
				args[i] = strings.ReplaceAll(args[i], "/usr/bin", f.binDir)
				args[i] = strings.ReplaceAll(args[i], etcFRR, f.configDir)
			}

			args[2] = saveWrapperInterposers + args[2]
			process := exec.CommandContext(ctx, args[0], args[1:]...)
			process.WaitDelay = time.Second
			out, err := process.CombinedOutput()
			res := clabexec.NewExecResult(cmd)
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					res.SetReturnCode(exit.ExitCode())
					res.SetStdErr(out)

					return res, nil
				}

				return nil, err
			}

			return res, nil
		})

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	return n.installConfigSaveWrapper(ctx)
}

func (f saveWrapperFixture) assertCallable(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.path, "-b", "two words")
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "-b\ntwo words\n" {
		t.Fatalf("canonical vtysh is unavailable: output=%q error=%v", out, err)
	}
}

func (f saveWrapperFixture) assertBackup(t *testing.T) {
	t.Helper()

	data, err := os.ReadFile(f.path + ".containerlab")
	if err != nil || string(data) != string(f.original) {
		t.Fatalf("original backup changed or incomplete: %q, error=%v", data, err)
	}
}

func (f saveWrapperFixture) assertNoTemporaryFiles(t *testing.T) {
	t.Helper()

	if files := f.temporaryFiles(t); files != "" {
		t.Errorf("temporary installation files were not cleaned up: %s", files)
	}
}

func (f saveWrapperFixture) temporaryFiles(t *testing.T) string {
	t.Helper()

	files, err := os.ReadDir(f.binDir)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, file := range files {
		if file.Name() != "vtysh" && file.Name() != "vtysh.containerlab" {
			names = append(names, file.Name())
		}
	}

	return strings.Join(names, "\n")
}

func TestConfigSaveWrapperRemainsExecutableDuringInstall(t *testing.T) {
	f := newSaveWrapperFixture(t)
	for range 2 {
		if err := f.install(t); err != nil {
			t.Fatal(err)
		}

		f.assertCallable(t)
		f.assertBackup(t)
		f.assertNoTemporaryFiles(t)
	}

	data, err := os.ReadFile(os.Getenv("FRR_TEST_PROBES"))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"link:0", "chmod:0", "publish:0", "chmod:0", "publish:0"}
	if diff := cmp.Diff(want, strings.Fields(string(data))); diff != "" {
		t.Errorf("vtysh probes between installation operations (-want +got):\n%s", diff)
	}
}

func TestConfigSaveWrapperInstallationFailureAndRetry(t *testing.T) {
	for _, fault := range []string{"link", "link-interrupted", "write", "chmod", "publish"} {
		t.Run(fault, func(t *testing.T) {
			f := newSaveWrapperFixture(t)
			t.Setenv("FRR_TEST_FAULT", fault)
			if err := f.install(t); err == nil || !strings.Contains(err.Error(), "router1") {
				t.Fatalf("installation error = %v, want a node-specific failure", err)
			}

			f.assertCallable(t)
			if fault == "link" {
				if _, err := os.Stat(f.path + ".containerlab"); !os.IsNotExist(err) {
					t.Fatalf("failed link published a backup: %v", err)
				}
			} else {
				// Link creation publishes the complete original atomically;
				// even SIGKILL immediately afterward leaves a usable backup.
				f.assertBackup(t)
			}

			f.assertNoTemporaryFiles(t)

			t.Setenv("FRR_TEST_FAULT", "")
			if err := f.install(t); err != nil {
				t.Fatalf("retry failed: %v", err)
			}

			f.assertCallable(t)
			f.assertBackup(t)
			f.assertNoTemporaryFiles(t)
		})
	}
}

func TestConfigSaveWrapperReinstallationFailure(t *testing.T) {
	for _, fault := range []string{"write", "chmod", "publish"} {
		t.Run(fault, func(t *testing.T) {
			f := newSaveWrapperFixture(t)
			if err := f.install(t); err != nil {
				t.Fatal(err)
			}

			before, err := os.ReadFile(f.path)
			if err != nil {
				t.Fatal(err)
			}

			t.Setenv("FRR_TEST_FAULT", fault)
			if err := f.install(t); err == nil {
				t.Fatal("reinstallation unexpectedly succeeded")
			}

			after, err := os.ReadFile(f.path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("failed reinstall changed the working wrapper: %v", err)
			}

			f.assertCallable(t)
			f.assertBackup(t)
			f.assertNoTemporaryFiles(t)
		})
	}
}

func TestConfigSaveWrapperPreservesOriginalMetadata(t *testing.T) {
	f := newSaveWrapperFixture(t)
	// Root can also exercise preservation of ownership different from the
	// installer, as with the frrvty group in the container image.
	if os.Geteuid() == 0 {
		if err := os.Chown(f.path, 12345, 12346); err != nil {
			t.Fatal(err)
		}
	}

	modified := time.Unix(1_700_000_000, 123_456_789)
	if err := os.Chtimes(f.path, modified, modified); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(f.path)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.install(t); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(f.path + ".containerlab")
	if err != nil {
		t.Fatal(err)
	}

	if before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		t.Errorf("backup mode/time changed: before=%v %v after=%v %v",
			before.Mode(), before.ModTime(), after.Mode(), after.ModTime())
	}
	if !os.SameFile(before, after) {
		t.Error("backup does not refer to the original fixture inode")
	}

	oldStat, oldOK := before.Sys().(*syscall.Stat_t)
	newStat, newOK := after.Sys().(*syscall.Stat_t)
	if !oldOK || !newOK || oldStat.Uid != newStat.Uid || oldStat.Gid != newStat.Gid {
		t.Error("backup ownership changed or could not be inspected")
	}
}

func TestConfigSaveWrapperPreservesSymlink(t *testing.T) {
	for _, relative := range []bool{false, true} {
		name := "absolute"
		if relative {
			name = "relative"
		}
		t.Run(name, func(t *testing.T) {
			f := newSaveWrapperFixture(t)
			target := filepath.Join(filepath.Dir(f.binDir), "original-vtysh")
			if err := os.Rename(f.path, target); err != nil {
				t.Fatal(err)
			}
			linkTarget := target
			if relative {
				linkTarget = "../original-vtysh"
			}
			if err := os.Symlink(linkTarget, f.path); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(f.path)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := f.install(t); err != nil {
					t.Fatal(err)
				}
				after, err := os.Lstat(f.path + ".containerlab")
				if err != nil || !os.SameFile(before, after) {
					t.Fatalf("backup did not retain the original symlink: %v", err)
				}
				got, err := os.Readlink(f.path + ".containerlab")
				if err != nil || got != linkTarget {
					t.Fatalf("backup link target = %q, error=%v, want %q", got, err, linkTarget)
				}
				f.assertCallable(t)
				f.assertBackup(t)
				f.assertNoTemporaryFiles(t)
			}
		})
	}
}

func TestConfigSaveWrapperRejectsConflictingBackup(t *testing.T) {
	for _, kind := range []string{"directory", "directory-symlink", "dangling-symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newSaveWrapperFixture(t)
			backup := f.path + ".containerlab"
			target := filepath.Join(filepath.Dir(f.binDir), "backup-target")
			if kind != "dangling-symlink" {
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "directory" {
				if err := os.Rename(target, backup); err != nil {
					t.Fatal(err)
				}
				target = backup
			} else if err := os.Symlink(target, backup); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(backup)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.install(t); err == nil || !strings.Contains(err.Error(), "router1") {
				t.Fatalf("installation error = %v, want a node-specific failure", err)
			}
			after, err := os.Lstat(backup)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("conflicting backup was changed: %v", err)
			}
			if kind != "dangling-symlink" {
				entries, err := os.ReadDir(target)
				if err != nil || len(entries) != 0 {
					t.Fatalf("backup directory was modified: entries=%v error=%v", entries, err)
				}
			}
			data, err := os.ReadFile(f.path)
			if err != nil || string(data) != string(f.original) {
				t.Fatalf("failed installation replaced canonical vtysh: %v", err)
			}
			f.assertCallable(t)
			f.assertNoTemporaryFiles(t)
		})
	}
}

func TestConfigSaveWrapperPreservesExtendedAttributes(t *testing.T) {
	// Linux POSIX ACL version 2: owner, named user, owning group, mask, other.
	acl := binary.LittleEndian.AppendUint32(nil, 2)
	for _, entry := range []struct {
		tag, permissions uint16
		id               uint32
	}{
		{0x01, 7, 0xffffffff},
		{0x02, 4, 12345},
		{0x04, 5, 0xffffffff},
		{0x10, 5, 0xffffffff},
		{0x20, 1, 0xffffffff},
	} {
		acl = binary.LittleEndian.AppendUint16(acl, entry.tag)
		acl = binary.LittleEndian.AppendUint16(acl, entry.permissions)
		acl = binary.LittleEndian.AppendUint32(acl, entry.id)
	}
	// Version-2 file capability: permitted CAP_NET_BIND_SERVICE, effective flag.
	capability := make([]byte, 20)
	binary.LittleEndian.PutUint32(capability, 0x02000001)
	binary.LittleEndian.PutUint32(capability[4:], 1<<10)
	for _, tc := range []struct {
		name  string
		value []byte
	}{
		{"user.containerlab-test", []byte("preserve original metadata")},
		{"system.posix_acl_access", acl},
		{"security.capability", capability},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSaveWrapperFixture(t)
			if err := unix.Setxattr(f.path, tc.name, tc.value, 0); err != nil {
				if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EPERM) ||
					errors.Is(err, unix.EACCES) {
					t.Skipf("cannot set %s in this test environment: %v", tc.name, err)
				}
				t.Fatal(err)
			}
			before := readSaveWrapperXattr(t, f.path, tc.name)
			for range 2 {
				if err := f.install(t); err != nil {
					t.Fatal(err)
				}
				after := readSaveWrapperXattr(t, f.path+".containerlab", tc.name)
				if diff := cmp.Diff(before, after); diff != "" {
					t.Errorf("backup %s changed (-want +got):\n%s", tc.name, diff)
				}
				f.assertCallable(t)
				f.assertBackup(t)
				f.assertNoTemporaryFiles(t)
			}
		})
	}
}

func readSaveWrapperXattr(t *testing.T, path, name string) []byte {
	t.Helper()

	size, err := unix.Getxattr(path, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	value := make([]byte, size)
	n, err := unix.Getxattr(path, name, value)
	if err != nil {
		t.Fatal(err)
	}

	return value[:n]
}
