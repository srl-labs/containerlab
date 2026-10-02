package tailscale

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestSidecarName(t *testing.T) {
	if got := SidecarName("srl1"); got != "srl1-ts" {
		t.Fatalf("SidecarName = %q, want srl1-ts", got)
	}
}

func TestApplyTailscaleEnv(t *testing.T) {
	cfg := &clabtypes.NodeConfig{
		ShortName: "srl1-ts",
		Labels: map[string]string{
			clabconstants.Containerlab: "mylab",
			ParentLabel:                "srl1",
		},
	}
	mgmt := &clabtypes.MgmtNet{
		Tailscale: &clabtypes.TailscaleConfig{
			AuthKey: "tskey-auth-test",
		},
	}

	applyTailscaleEnv(cfg, mgmt)

	want := map[string]string{
		"TS_USERSPACE":  "false",
		"TS_ACCEPT_DNS": "false",
		"TS_HOSTNAME":   "clab-mylab-srl1",
		"TS_AUTHKEY":    "file:/clab/authkey",
	}
	for k, v := range want {
		if cfg.Env[k] != v {
			t.Errorf("env %s = %q, want %q", k, cfg.Env[k], v)
		}
	}
	if _, ok := cfg.Env["TS_STATE_DIR"]; ok {
		t.Fatalf("TS_STATE_DIR = %q; sidecars must keep state in memory to stay ephemeral",
			cfg.Env["TS_STATE_DIR"])
	}
}

func TestWriteAuthKey(t *testing.T) {
	mgmt := &clabtypes.MgmtNet{
		Tailscale: &clabtypes.TailscaleConfig{AuthKey: "tskey-auth-test"},
	}
	cfg := &clabtypes.NodeConfig{
		LabDir: t.TempDir(),
		Labels: map[string]string{ParentLabel: "n1"},
	}
	applyTailscaleEnv(cfg, mgmt)
	if err := writeAuthKey(cfg, mgmt); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg.LabDir, authKeyFile)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "tskey-auth-test" {
		t.Fatalf("authkey = %q", b)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("authkey mode = %o, want 600", st.Mode().Perm())
	}

	custom := &clabtypes.NodeConfig{
		LabDir: t.TempDir(),
		Env:    map[string]string{"TS_AUTHKEY": "user-key"},
	}
	if err := writeAuthKey(custom, mgmt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(custom.LabDir, authKeyFile)); !os.IsNotExist(err) {
		t.Fatalf("authkey written despite user TS_AUTHKEY: %v", err)
	}
}

func TestApplyTailscaleEnvKeepsExisting(t *testing.T) {
	cfg := &clabtypes.NodeConfig{
		ShortName: "n1-ts",
		Env: map[string]string{
			"TS_HOSTNAME": "custom",
			"TS_AUTHKEY":  "already-set",
		},
		Labels: map[string]string{
			clabconstants.Containerlab: "lab",
			ParentLabel:                "n1",
		},
	}
	applyTailscaleEnv(cfg, &clabtypes.MgmtNet{
		Tailscale: &clabtypes.TailscaleConfig{AuthKey: "other"},
	})

	if cfg.Env["TS_HOSTNAME"] != "custom" {
		t.Fatalf("TS_HOSTNAME overwritten: %q", cfg.Env["TS_HOSTNAME"])
	}
	if cfg.Env["TS_AUTHKEY"] != "already-set" {
		t.Fatalf("TS_AUTHKEY overwritten: %q", cfg.Env["TS_AUTHKEY"])
	}
}

func TestWriteAuthKeyReplacesExistingFile(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		cfg := &clabtypes.NodeConfig{LabDir: t.TempDir()}
		mgmt := &clabtypes.MgmtNet{Tailscale: &clabtypes.TailscaleConfig{AuthKey: "secret"}}
		applyTailscaleEnv(cfg, mgmt)
		path := filepath.Join(cfg.LabDir, authKeyFile)
		target := path
		if symlink {
			target = filepath.Join(t.TempDir(), "unrelated")
		}
		if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		if symlink {
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}
		if err := writeAuthKey(cfg, mgmt); err != nil {
			t.Fatal(err)
		}
		st, err := os.Lstat(path)
		if err != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0o600 {
			t.Fatalf("auth-key file must be a regular file with mode 600: %v, %v", st, err)
		}
		if symlink {
			b, err := os.ReadFile(target)
			if err != nil || string(b) != "original" {
				t.Fatalf("symlink target changed: %q, %v", b, err)
			}
		}
	}
}

func TestDeployRefreshesParentManagementIP(t *testing.T) {
	n, rt := newTestSidecar(t)
	parent := clabmocksmocknodes.NewMockNode(gomock.NewController(t))
	parentCfg := &clabtypes.NodeConfig{}
	parent.EXPECT().Config().Return(parentCfg).AnyTimes()
	parent.EXPECT().
		UpdateConfigWithRuntimeInfo(gomock.Any()).
		DoAndReturn(func(context.Context) error {
			parentCfg.MgmtIPv4Address = "172.20.20.2"
			return nil
		})
	rt.EXPECT().CreateContainer(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cfg *clabtypes.NodeConfig) (string, error) {
			if cfg.Env["TS_DEST_IP"] != parentCfg.MgmtIPv4Address {
				t.Fatalf("TS_DEST_IP = %q", cfg.Env["TS_DEST_IP"])
			}
			return "id", nil
		})
	rt.EXPECT().StartContainer(gomock.Any(), "id", &n.DefaultNode)
	if err := n.Deploy(context.Background(), &clabnodes.DeployParams{
		Nodes: map[string]clabnodes.Node{"n1": parent},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestWriteServeConfig(t *testing.T) {
	dir := t.TempDir()
	n1 := new(tailscale)
	n1.Cfg = &clabtypes.NodeConfig{
		MgmtIPv4Address: "172.20.20.2",
		TailscalePorts: []clabtypes.TailscalePort{
			{Listen: 8022, Dest: 22},
			{Listen: 8574, Dest: 57400},
		},
	}
	if err := WriteServeConfig(dir, map[string]clabnodes.Node{"n1": n1}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ServeFile))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"TCP":{"8022":{"TCPForward":"172.20.20.2:22"},"8574":{"TCPForward":"172.20.20.2:57400"}}}`
	if string(b) != want {
		t.Fatalf("serve.json = %s, want %s", b, want)
	}
}

func TestProxyConfig(t *testing.T) {
	cfg := ProxyConfig("mylab", "clab-mylab-ts", "clab", "/tmp/clab-mylab/ts")
	if cfg.ShortName != ProxyName || cfg.LongName != "clab-mylab-ts" || cfg.MgmtNet != "clab" {
		t.Fatalf("names = %q %q %q", cfg.ShortName, cfg.LongName, cfg.MgmtNet)
	}
	if len(cfg.Binds) != 1 || cfg.Binds[0] != "/tmp/clab-mylab/ts:"+tsStateDir {
		t.Fatalf("binds = %q", cfg.Binds)
	}
	if len(cfg.CapAdd) != 0 || len(cfg.Devices) != 0 {
		t.Fatalf("userspace proxy has cap-add %q, devices %q", cfg.CapAdd, cfg.Devices)
	}
	want := map[string]string{
		"TS_USERSPACE":    "true",
		"TS_HOSTNAME":     "clab-mylab",
		"TS_SERVE_CONFIG": tsStateDir + "/" + ServeFile,
	}
	for k, v := range want {
		if cfg.Env[k] != v {
			t.Errorf("env %s = %q, want %q", k, cfg.Env[k], v)
		}
	}
	if _, ok := cfg.Env["TS_AUTHKEY"]; ok {
		t.Fatal("TS_AUTHKEY set on SSO proxy")
	}
}

func TestParentMgmtIP(t *testing.T) {
	parent := new(tailscale)
	parent.Cfg = &clabtypes.NodeConfig{
		MgmtIPv4Address: "172.20.20.2",
		MgmtIPv6Address: "2001:db8::2",
	}
	cfg := &clabtypes.NodeConfig{
		Labels: map[string]string{ParentLabel: "n1"},
	}
	params := &clabnodes.DeployParams{
		Nodes: map[string]clabnodes.Node{"n1": parent},
	}

	if got := parentMgmtIP(cfg, params); got != "172.20.20.2" {
		t.Fatalf("parentMgmtIP = %q, want 172.20.20.2", got)
	}

	parent.Cfg.MgmtIPv4Address = ""
	if got := parentMgmtIP(cfg, params); got != "2001:db8::2" {
		t.Fatalf("parentMgmtIP v6 = %q, want 2001:db8::2", got)
	}

	if got := parentMgmtIP(cfg, nil); got != "" {
		t.Fatalf("nil params = %q", got)
	}
	if got := parentMgmtIP(&clabtypes.NodeConfig{
		Labels: map[string]string{ParentLabel: "missing"},
	}, params); got != "" {
		t.Fatalf("missing parent = %q", got)
	}
}

func TestInitDefaults(t *testing.T) {
	n := new(tailscale)
	cfg := &clabtypes.NodeConfig{
		ShortName: "n1-ts",
		LabDir:    "/tmp/clab/n1-ts",
		Labels:    map[string]string{},
	}
	if err := n.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Image != DefaultImage {
		t.Fatalf("image = %q, want %q", cfg.Image, DefaultImage)
	}
	if cfg.RestartPolicy != "always" {
		t.Fatalf("restart-policy = %q, want always", cfg.RestartPolicy)
	}
	if len(cfg.CapAdd) != 1 || cfg.CapAdd[0] != "NET_ADMIN" {
		t.Fatalf("cap-add = %q, want [NET_ADMIN]", cfg.CapAdd)
	}
	if len(cfg.Devices) != 1 || cfg.Devices[0] != "/dev/net/tun" {
		t.Fatalf("devices = %q, want [/dev/net/tun]", cfg.Devices)
	}
	if cfg.Sysctls["net.ipv4.ip_forward"] != "1" {
		t.Fatalf("ipv4 forward = %q", cfg.Sysctls["net.ipv4.ip_forward"])
	}
	if cfg.Sysctls["net.ipv6.conf.all.forwarding"] != "1" {
		t.Fatalf("ipv6 forward = %q", cfg.Sysctls["net.ipv6.conf.all.forwarding"])
	}
	wantBind := cfg.LabDir + ":" + sidecarLabDir
	if len(cfg.Binds) != 1 || cfg.Binds[0] != wantBind {
		t.Fatalf("binds = %q, want [%s]", cfg.Binds, wantBind)
	}
}

func newTestSidecar(t *testing.T) (*tailscale, *clabmocksmockruntime.MockContainerRuntime) {
	t.Helper()
	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
	n := new(tailscale)
	cfg := &clabtypes.NodeConfig{
		ShortName: "n1-ts",
		LongName:  "clab-mylab-n1-ts",
		Labels:    map[string]string{ParentLabel: "n1"},
	}
	if err := n.Init(cfg, clabnodes.WithRuntime(rt)); err != nil {
		t.Fatal(err)
	}

	return n, rt
}

func TestPreDestroyLogsOut(t *testing.T) {
	n, rt := newTestSidecar(t)
	logout := []string{"tailscale", "--socket=" + tsSocket, "logout"}

	rt.EXPECT().Exec(gomock.Any(), "clab-mylab-n1-ts", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			if !slices.Equal(cmd.GetCmd(), logout) {
				t.Fatalf("exec = %q, want %q", cmd.GetCmd(), logout)
			}
			return clabexec.NewExecResult(cmd), nil
		})

	if err := n.PreDestroy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPreDestroyIgnoresLogoutFailure(t *testing.T) {
	n, rt := newTestSidecar(t)
	rt.EXPECT().Exec(gomock.Any(), "clab-mylab-n1-ts", gomock.Any()).
		Return(nil, errors.New("container is not running"))

	if err := n.PreDestroy(context.Background()); err != nil {
		t.Fatalf("PreDestroy = %v, want nil", err)
	}
}
