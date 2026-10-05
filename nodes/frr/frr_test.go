// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package frr

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/ssh"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// newTestNode returns an frr node rooted at a temporary lab directory.
func newTestNode(t *testing.T, cfg *clabtypes.NodeConfig) *frr {
	t.Helper()

	cfg.LabDir = filepath.Join(t.TempDir(), cfg.ShortName)
	if cfg.Sysctls == nil {
		cfg.Sysctls = map[string]string{}
	}

	n := new(frr)
	if err := n.Init(cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return n
}

func readConfigFile(t *testing.T, n *frr, name string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(n.Cfg.LabDir, cfgDir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	return string(b)
}

// Init must mount all three config files, since the image ships neither
// frr.conf nor vtysh.conf and vtysh will not start without them.
// The config directory is mounted as a directory, not as three separate files:
// FRR renames frr.conf to frr.conf.sav when saving, and a single-file bind mount
// cannot be renamed.
func TestInitBindsConfigDirectory(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	want := filepath.Join(n.Cfg.LabDir, cfgDir) + ":" + etcFRR

	found := false

	for _, b := range n.Cfg.Binds {
		if b == want {
			found = true
			break
		}

		// A per-file mount is what this replaces, and it is the failure worth
		// naming: it looks like it works until the first "write memory".
		if strings.HasPrefix(b, filepath.Join(n.Cfg.LabDir, cfgDir)+"/") {
			t.Errorf("per-file bind mount %q; the directory must be mounted instead", b)
		}
	}

	if !found {
		t.Errorf("no bind mount %q, got %v", want, n.Cfg.Binds)
	}
}

func TestInitEnablesForwarding(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	for k, want := range map[string]string{
		"net.ipv4.ip_forward":          "1",
		"net.ipv6.conf.all.forwarding": "1",
	} {
		if got := n.Cfg.Sysctls[k]; got != want {
			t.Errorf("sysctl %s = %q, want %q", k, got, want)
		}
	}
}

func TestCreateFRRFilesWithoutStartupConfig(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	if err := n.createFRRFiles(); err != nil {
		t.Fatalf("createFRRFiles: %v", err)
	}

	got := readConfigFile(t, n, vtyshConfFile)
	if !strings.Contains(got, "service integrated-vtysh-config") {
		t.Errorf("vtysh.conf = %q, want integrated config enabled", got)
	}

	// The bundled default, since no startup-config was given.
	got = readConfigFile(t, n, frrConfFile)
	if !strings.Contains(got, "frr defaults traditional") {
		t.Errorf("frr.conf = %q, want the bundled default", got)
	}

	// No daemon list means every daemon runs.
	daemons := readConfigFile(t, n, daemonsFile)
	for _, d := range configurableDaemons {
		if !strings.Contains(daemons, d+"=yes") {
			t.Errorf("daemon %s is not enabled by default", d)
		}
	}
}

func TestCreateFRRFilesUsesStartupConfig(t *testing.T) {
	startup := filepath.Join(t.TempDir(), "router1.conf")

	want := "router ospf\n network 10.0.0.0/8 area 0\n"
	if err := os.WriteFile(startup, []byte(want), 0o644); err != nil {
		t.Fatalf("writing startup config: %v", err)
	}

	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName:     "router1",
		StartupConfig: startup,
	})

	if err := n.createFRRFiles(); err != nil {
		t.Fatalf("createFRRFiles: %v", err)
	}

	if got := readConfigFile(t, n, frrConfFile); got != want {
		t.Errorf("frr.conf = %q, want %q", got, want)
	}
}

// The daemon list set in the topology must reach the rendered daemons file.
func TestCreateFRRFilesHonoursExtras(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName: "router1",
		Extras: &clabtypes.Extras{
			FRR: &clabtypes.FRRExtras{Daemons: []string{"ospfd", "bfdd"}},
		},
	})

	if err := n.createFRRFiles(); err != nil {
		t.Fatalf("createFRRFiles: %v", err)
	}

	daemons := readConfigFile(t, n, daemonsFile)

	for _, d := range []string{"ospfd", "bfdd"} {
		if !strings.Contains(daemons, d+"=yes") {
			t.Errorf("daemon %s should be enabled", d)
		}
	}

	for _, d := range []string{"bgpd", "isisd", "pimd"} {
		if !strings.Contains(daemons, d+"=no") {
			t.Errorf("daemon %s should be disabled", d)
		}
	}
}

// An unusable daemon name must fail the deploy rather than silently produce a
// node with the wrong daemons running.
func TestCreateFRRFilesRejectsUnknownDaemon(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName: "router1",
		Extras: &clabtypes.Extras{
			FRR: &clabtypes.FRRExtras{Daemons: []string{"bogusd"}},
		},
	})

	err := n.createFRRFiles()
	if err == nil {
		t.Fatal("expected an error for an unknown daemon, got none")
	}

	for _, want := range []string{"bogusd", "router1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestRegisterKindNames(t *testing.T) {
	r := clabnodes.NewNodeRegistry()
	Register(r)

	for _, name := range []string{"frr", "frrouting"} {
		if _, err := r.NewNodeOfKind(name); err != nil {
			t.Errorf("kind %q is not registered: %v", name, err)
		}
	}
}

// The username is what containerlab writes into the generated ssh config and
// the ansible and nornir inventories, so a bare "ssh <node>" reaches vtysh.
func TestRegisterCredentials(t *testing.T) {
	r := clabnodes.NewNodeRegistry()
	Register(r)

	creds := r.Kind("frr").GetCredentials()

	if got := creds.GetUsername(); got != "admin" {
		t.Errorf("username = %q, want %q", got, "admin")
	}

	if got := creds.GetPassword(); got != "admin" {
		t.Errorf("password = %q, want %q", got, "admin")
	}
}

// Exercise the shell command with multiple keys: Go string quoting must not
// turn the newline separators into literal backslash-n sequences.
func TestAddSSHKeysWritesMultipleSSHKeys(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	var want strings.Builder

	for range 2 {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}

		key, err := ssh.NewPublicKey(pub)
		if err != nil {
			t.Fatal(err)
		}

		n.sshPubKeys = append(n.sshPubKeys, key)
		want.Write(ssh.MarshalAuthorizedKey(key))
	}

	dir := t.TempDir()
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n.WithRuntime(rt)
	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			args := append([]string(nil), cmd.GetCmd()...)
			// Keep the test unprivileged and restrict writes to its temporary
			// directory. The existence guard goes too, so that both users are
			// exercised although the host running the test has no admin.
			args[2] = strings.ReplaceAll(args[2], authzKeysTargets,
				"root:"+filepath.Join(dir, "root")+" admin:"+filepath.Join(dir, "admin"))
			args[2] = strings.ReplaceAll(args[2], authzKeysGuard, "true")
			args[2] = strings.ReplaceAll(args[2], authzKeysChown, "true")

			out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
			if err != nil {
				t.Fatalf("SSH key script: %v: %s", err, out)
			}

			return clabexec.NewExecResult(cmd), nil
		})

	if err := n.addSSHKeys(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, user := range []string{"root", "admin"} {
		data, err := os.ReadFile(filepath.Join(dir, user, ".ssh", "authorized_keys"))
		if err != nil {
			t.Fatal(err)
		}

		if string(data) != want.String() {
			t.Fatalf("%s authorized_keys = %q, want %q", user, data, want.String())
		}
	}
}

// A plain release image has no admin user, and the keys still have to reach the
// users that do exist rather than the script failing on the first absent one.
func TestAddSSHKeysSkipsAbsentUser(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	n.sshPubKeys = append(n.sshPubKeys, key)

	dir := t.TempDir()
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n.WithRuntime(rt)
	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			args := append([]string(nil), cmd.GetCmd()...)
			// root stands in for the user that exists and "nosuchuser" for the
			// one that does not, so the real guard is what is under test here.
			args[2] = strings.ReplaceAll(args[2], authzKeysTargets,
				"root:"+filepath.Join(dir, "root")+" nosuchuser:"+filepath.Join(dir, "nosuchuser"))
			args[2] = strings.ReplaceAll(args[2], authzKeysChown, "true")

			out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
			if err != nil {
				t.Fatalf("SSH key script: %v: %s", err, out)
			}

			return clabexec.NewExecResult(cmd), nil
		})

	if err := n.addSSHKeys(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "root", ".ssh", "authorized_keys")); err != nil {
		t.Errorf("keys not written for the user that exists: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "nosuchuser")); !os.IsNotExist(err) {
		t.Errorf("absent user was not skipped: %v", err)
	}
}

// FRR leaves frr.conf mode 0600 after a "write memory", and the file is bind
// mounted, so that mode lands on the lab directory copy. Both the deploy and
// the save path have to put it back.
func TestConfigFilePermissionsRestored(t *testing.T) {
	const saved = "frr defaults traditional\nrouter bgp 65000\nexit\n"

	writeLockedDown := func(t *testing.T, n *frr) string {
		t.Helper()

		dir := filepath.Join(n.Cfg.LabDir, cfgDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(dir, frrConfFile)
		if err := os.WriteFile(path, []byte(saved), 0o600); err != nil {
			t.Fatal(err)
		}

		return path
	}

	assertReadable := func(t *testing.T, path string) {
		t.Helper()

		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}

		if got := info.Mode().Perm(); got != clabconstants.PermissionsFileDefault {
			t.Errorf("mode = %#o, want %#o", got, clabconstants.PermissionsFileDefault)
		}
	}

	t.Run("on deploy", func(t *testing.T) {
		n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})
		path := writeLockedDown(t, n)

		if err := n.createFRRFiles(); err != nil {
			t.Fatal(err)
		}

		assertReadable(t, path)

		// The repair must not cost the user the configuration it repairs.
		if got := readConfigFile(t, n, frrConfFile); got != saved {
			t.Errorf("frr.conf = %q, want the saved config %q", got, saved)
		}
	})

	t.Run("on save", func(t *testing.T) {
		n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})
		path := writeLockedDown(t, n)

		rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
		n.WithRuntime(rt)
		rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
				res := clabexec.NewExecResult(cmd)
				res.SetStdOut([]byte(saved))

				return res, nil
			})

		if _, err := n.SaveConfig(context.Background()); err != nil {
			t.Fatal(err)
		}

		assertReadable(t, path)
	})
}

// containerlab's "save --copy" copies the file named by SaveConfigResult and
// silently skips a node that reports none, so the path has to come back.
func TestSaveConfigReportsPath(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	// SaveConfig writes into the config directory PreDeploy creates.
	if err := os.MkdirAll(filepath.Join(n.Cfg.LabDir, cfgDir), 0o755); err != nil {
		t.Fatal(err)
	}

	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n.WithRuntime(rt)
	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			res := clabexec.NewExecResult(cmd)
			res.SetStdOut([]byte("frr defaults traditional\n"))

			return res, nil
		})

	result, err := n.SaveConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if result == nil {
		t.Fatal("SaveConfig returned no result; --copy would skip this node")
	}

	want := filepath.Join(n.Cfg.LabDir, cfgDir, frrConfFile)
	if result.ConfigPath != want {
		t.Errorf("ConfigPath = %q, want %q", result.ConfigPath, want)
	}

	// The path is only useful if it names the file that was actually written.
	if _, err := os.Stat(result.ConfigPath); err != nil {
		t.Errorf("ConfigPath does not exist: %v", err)
	}
}

// runPasswordScript runs the command PostDeploy asked for on the host, with
// chpasswd replaced by a file that records what it would have been fed. keepGuard
// leaves the check for the user in place; without it the script runs as though
// the user exists, since the host running the test has no admin.
func runPasswordScript(
	ctx context.Context,
	t *testing.T,
	cmd *clabexec.ExecCmd,
	out string,
	keepGuard bool,
) {
	t.Helper()

	args := append([]string(nil), cmd.GetCmd()...)
	if !keepGuard {
		args[2] = strings.ReplaceAll(args[2], passwordGuard, "true")
	}

	args[2] = strings.ReplaceAll(args[2], "| chpasswd", "> "+out)

	if b, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("password script: %v: %s", err, b)
	}
}

// The image ships admin without a password, so the credentials only work for a
// password login once PostDeploy has set them -- including on a host with no
// ssh keys, where adding keys has nothing to do.
func TestSetPassword(t *testing.T) {
	// Every character here means something to a shell. It has to reach
	// chpasswd exactly as written.
	const password = `a'b"c$(touch /tmp/x);d` + "`e`" + `\f g`

	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName:   "router1",
		Credentials: clabtypes.NodeCredentials{Username: "admin", Password: password},
	})

	out := filepath.Join(t.TempDir(), "chpasswd-input")
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n.WithRuntime(rt)
	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			runPasswordScript(ctx, t, cmd, out, false)

			return clabexec.NewExecResult(cmd), nil
		})

	if err := n.setPassword(context.Background()); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	if want := "admin:" + password + "\n"; string(got) != want {
		t.Errorf("chpasswd input = %q, want %q", got, want)
	}
}

// A plain release image has no admin, and the password step must not fail the
// deploy there.
func TestSetPasswordSkipsAbsentUser(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName:   "router1",
		Credentials: clabtypes.NodeCredentials{Username: "nosuchuser", Password: "pw"},
	})

	out := filepath.Join(t.TempDir(), "chpasswd-input")
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n.WithRuntime(rt)
	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			runPasswordScript(ctx, t, cmd, out, true)

			return clabexec.NewExecResult(cmd), nil
		})

	if err := n.setPassword(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("chpasswd ran for a user the image does not have: %v", err)
	}
}

// chpasswd reads one user:password pair per line, so a line break in the
// password would set a second user's password. It is refused before anything
// runs in the container; the mock fails the test on any Exec.
func TestPostDeployRejectsPasswordWithLineBreak(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{
		ShortName:   "router1",
		Credentials: clabtypes.NodeCredentials{Username: "admin", Password: "pw\nroot:owned"},
	})

	n.WithRuntime(clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t)))

	err := n.PostDeploy(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("PostDeploy error = %v, want one about the line break", err)
	}
}

// PostDeploy installs the save wrapper even when authentication needs no work.
func TestPostDeployConfigSaveWrapper(t *testing.T) {
	n := newTestNode(t, &clabtypes.NodeConfig{ShortName: "router1"})

	dir := filepath.Join(t.TempDir(), "with spaces")
	binDir := filepath.Join(dir, "bin")
	configDir := filepath.Join(dir, "config")
	for _, path := range []string{binDir, configDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Stand in for FRR's writer without requiring privileges or a container.
	writer := `#!/bin/sh
case "$*" in
    -w|--writeconfig)
        [ -z "$FRR_TEST_FAILURE" ] || exit "$FRR_TEST_FAILURE"
        printf 'saved configuration\n' > "$FRR_TEST_CONFIG"
        chmod 600 "$FRR_TEST_CONFIG"
        ;;
    *)
        printf '%s\n' "$@"
        ;;
esac`
	path := filepath.Join(binDir, "vtysh")
	if err := os.WriteFile(path, []byte(writer), 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, frrConfFile)
	t.Setenv("FRR_TEST_CONFIG", configPath)
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n.WithRuntime(rt)
	rt.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			args := append([]string(nil), cmd.GetCmd()...)
			for _, i := range []int{2, 4} {
				args[i] = strings.ReplaceAll(args[i], "/usr/bin", binDir)
				args[i] = strings.ReplaceAll(args[i], etcFRR, configDir)
			}
			if out, err := exec.CommandContext(ctx, args[0], args[1:]...).
				CombinedOutput(); err != nil {
				t.Fatalf("install save wrapper: %v: %s", err, out)
			}

			return clabexec.NewExecResult(cmd), nil
		}).Times(2)

	// Reinstalling must retain the binary, rather than wrapping the wrapper.
	for _, arg := range []string{"-w", "--writeconfig"} {
		if err := n.PostDeploy(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(path, arg).CombinedOutput(); err != nil {
			t.Fatalf("save config: %v: %s", err, out)
		}
		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != clabconstants.PermissionsFileDefault {
			t.Errorf("mode = %#o, want %#o", got, clabconstants.PermissionsFileDefault)
		}
	}

	if out, err := exec.Command(path, "show version", "two words").CombinedOutput(); err != nil ||
		string(out) != "show version\ntwo words\n" {
		t.Fatalf("forwarded arguments: %q, error: %v", out, err)
	}
	t.Setenv("FRR_TEST_FAILURE", "42")
	if err := exec.Command(path, "-w").Run(); err == nil {
		t.Fatal("wrapper swallowed the binary's failure")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 42 {
		t.Fatalf("exit error = %v, want exit code 42", err)
	}
}
