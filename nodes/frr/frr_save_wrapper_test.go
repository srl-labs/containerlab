// Copyright 2026 RouteObjects
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package frr

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	clabexec "github.com/srl-labs/containerlab/exec"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// CHANGE: These interposers execute the real filesystem operations. Each probe models
// FRR starting vtysh between installer steps, without sleeps or production hooks.
const saveWrapperInterposers = `
frr_probe() {
    local status=0
    "$FRR_TEST_VTYSH" -b 'two words' > "$FRR_TEST_OUTPUT" 2>&1 || status=$?
    builtin printf '%s:%s\n' "$1" "$status" >> "$FRR_TEST_PROBES"
}
cp() {
    case "$FRR_TEST_FAULT" in
        copy|copy-interrupted)
            builtin printf 'partial copy' > "${@: -1}"
            [ "$FRR_TEST_FAULT" != copy-interrupted ] || kill -KILL $$
            return 91
            ;;
    esac
    command cp "$@" || return $?
    frr_probe copy
}
mv() {
    local stage=publish
    [ "${@: -1}" != "$FRR_TEST_VTYSH.containerlab" ] || stage=backup
    [ "$FRR_TEST_FAULT" != "$stage" ] || return 91
    command mv "$@" || return $?
    frr_probe "$stage"
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

	want := []string{"copy:0", "backup:0", "chmod:0", "publish:0", "chmod:0", "publish:0"}
	if diff := cmp.Diff(want, strings.Fields(string(data))); diff != "" {
		t.Errorf("vtysh probes between installation operations (-want +got):\n%s", diff)
	}
}

func TestConfigSaveWrapperInstallationFailureAndRetry(t *testing.T) {
	for _, fault := range []string{"copy", "copy-interrupted", "backup", "write", "chmod", "publish"} {
		t.Run(fault, func(t *testing.T) {
			f := newSaveWrapperFixture(t)
			t.Setenv("FRR_TEST_FAULT", fault)
			if err := f.install(t); err == nil || !strings.Contains(err.Error(), "router1") {
				t.Fatalf("installation error = %v, want a node-specific failure", err)
			}

			f.assertCallable(t)
			switch fault {
			case "copy", "copy-interrupted", "backup":
				if _, err := os.Stat(f.path + ".containerlab"); !os.IsNotExist(err) {
					t.Fatalf("incomplete backup was published: %v", err)
				}
			default:
				f.assertBackup(t)
			}

			// SIGKILL cannot run cleanup; abandoned temporary files must not
			// become the original backup or interfere with the next attempt.
			if fault != "copy-interrupted" {
				f.assertNoTemporaryFiles(t)
			}

			leftovers := f.temporaryFiles(t)
			if fault == "copy-interrupted" && leftovers == "" {
				t.Fatal("interrupted-copy fixture did not leave a temporary copy")
			}

			t.Setenv("FRR_TEST_FAULT", "")
			if err := f.install(t); err != nil {
				t.Fatalf("retry failed: %v", err)
			}

			f.assertCallable(t)
			f.assertBackup(t)
			if fault != "copy-interrupted" {
				f.assertNoTemporaryFiles(t)
			} else if got := f.temporaryFiles(t); got != leftovers {
				t.Errorf(
					"retry changed abandoned temporary files: before=%q after=%q",
					leftovers,
					got,
				)
			}
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

	// The image's BusyBox cp preserves timestamps at whole-second precision,
	// which also matches the packaged executable's timestamp.
	modified := time.Unix(1_700_000_000, 0)
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

	oldStat, oldOK := before.Sys().(*syscall.Stat_t)
	newStat, newOK := after.Sys().(*syscall.Stat_t)
	if !oldOK || !newOK || oldStat.Uid != newStat.Uid || oldStat.Gid != newStat.Gid {
		t.Error("backup ownership changed or could not be inspected")
	}
}
