// Copyright 2026 Abel Perez, Eluzmar Alviarez
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package light_olt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestKindNames(t *testing.T) {
	want := []string{"light_olt"}
	if !reflect.DeepEqual(kindNames, want) {
		t.Fatalf("kindNames = %#v, want %#v", kindNames, want)
	}
}

func TestRegister(t *testing.T) {
	registry := clabnodes.NewNodeRegistry()
	Register(registry)

	entry := registry.Kind("light_olt")
	if entry == nil {
		t.Fatal("light_olt was not registered")
	}

	credentials := entry.GetCredentials()
	if credentials == nil || credentials.GetUsername() != "admin" || credentials.GetPassword() != "admin" {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}

	generate := entry.GetGenerateAttributes()
	if generate == nil {
		t.Fatal("missing generate attributes")
	}
	if !generate.IsGenerateable() || generate.GetInterfaceFormat() != "eth%d" {
		t.Fatalf("unexpected generate attributes: generateable=%v format=%q", generate.IsGenerateable(), generate.GetInterfaceFormat())
	}
}

func TestInitDefaults(t *testing.T) {
	labDir := t.TempDir()
	cfg := &clabtypes.NodeConfig{ShortName: "olt1", LabDir: labDir}
	node := &lightOLT{}

	if err := node.Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if cfg.ShmSize != defaultShmSize {
		t.Fatalf("ShmSize = %q, want %q", cfg.ShmSize, defaultShmSize)
	}
	if cfg.RestartPolicy != "no" {
		t.Fatalf("RestartPolicy = %q, want no", cfg.RestartPolicy)
	}
	if cfg.Env == nil {
		t.Fatal("Env was not initialized")
	}
	// The startup path is only decided in PreDeploy, so Init must not leave a
	// provisional value behind.
	if got, ok := cfg.Env[startupConfigEnv]; ok {
		t.Fatalf("%s was set during Init: %q", startupConfigEnv, got)
	}
	if got := cfg.Env[enforceStartupConfigEnv]; got != "false" {
		t.Fatalf("%s = %q", enforceStartupConfigEnv, got)
	}
	wantConfigPath := filepath.Join(labDir, configDirName, startupConfigFileName)
	if cfg.ResStartupConfig != wantConfigPath {
		t.Fatalf("ResStartupConfig = %q, want %q", cfg.ResStartupConfig, wantConfigPath)
	}
	wantBind := filepath.Join(labDir, configDirName) + ":" + configDirDst
	if !containsString(cfg.Binds, wantBind) {
		t.Fatalf("Binds = %#v, want %q", cfg.Binds, wantBind)
	}
	if cfg.Healthcheck == nil {
		t.Fatal("Healthcheck was not initialized")
	}
	if got := cfg.Healthcheck.Test; !reflect.DeepEqual(got, []string{"CMD", healthcheckCommand}) {
		t.Fatalf("Healthcheck.Test = %#v", got)
	}
	if got := node.LinkApplyMode(context.Background()); got != clabnodes.LinkApplyModeLive {
		t.Fatalf("LinkApplyMode() = %q, want %q", got, clabnodes.LinkApplyModeLive)
	}
}

func TestInitIsIdempotent(t *testing.T) {
	labDir := t.TempDir()
	cfg := &clabtypes.NodeConfig{ShortName: "olt1", LabDir: labDir}

	for i := 0; i < 2; i++ {
		node := &lightOLT{}
		if err := node.Init(cfg); err != nil {
			t.Fatalf("Init() call %d error = %v", i+1, err)
		}
	}

	wantBind := filepath.Join(labDir, configDirName) + ":" + configDirDst
	count := 0
	for _, bind := range cfg.Binds {
		if bind == wantBind {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("bind %q appears %d times in %#v, want once", wantBind, count, cfg.Binds)
	}
}

func TestInitRejectsUnsupportedStartupConfig(t *testing.T) {
	tests := map[string]struct {
		startupConfig string
		wantError     bool
	}{
		"tgz bundle":         {startupConfig: "olt.tgz"},
		"txt overlay":        {startupConfig: "olt.txt"},
		"uppercase overlay":  {startupConfig: "OLT.TXT"},
		"uppercase bundle":   {startupConfig: "OLT.TGZ"},
		"no startup config":  {startupConfig: ""},
		"tar.gz bundle":      {startupConfig: "olt.tar.gz", wantError: true},
		"cfg file":           {startupConfig: "olt.cfg", wantError: true},
		"extensionless file": {startupConfig: "olt", wantError: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			node := &lightOLT{}
			err := node.Init(&clabtypes.NodeConfig{
				ShortName:     "olt1",
				LabDir:        t.TempDir(),
				StartupConfig: tc.startupConfig,
			})
			if tc.wantError && err == nil {
				t.Fatalf("Init() accepted startup-config %q", tc.startupConfig)
			}
			if !tc.wantError && err != nil {
				t.Fatalf("Init() rejected startup-config %q: %v", tc.startupConfig, err)
			}
		})
	}
}

func TestPrepareStartupConfig(t *testing.T) {
	labDir := t.TempDir()
	source := filepath.Join(t.TempDir(), "saved.tgz")
	want := []byte("multi-plane bundle")
	if err := os.WriteFile(source, want, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &clabtypes.NodeConfig{
		ShortName:     "olt1",
		LabDir:        labDir,
		StartupConfig: source,
	}
	node := &lightOLT{}
	if err := node.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(node.configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := node.prepareStartupConfig(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := cfg.Env[startupConfigEnv]; got != "/clab/config/light-olt-startup.tgz" {
		t.Fatalf("%s = %q", startupConfigEnv, got)
	}
	got, err := os.ReadFile(node.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("copied startup-config = %q, want %q", got, want)
	}
}

func TestPrepareStartupConfigIsANoopWhenSourceIsDestination(t *testing.T) {
	labDir := t.TempDir()
	cfg := &clabtypes.NodeConfig{ShortName: "olt1", LabDir: labDir}
	node := &lightOLT{}
	if err := node.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(node.configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Point the topology at the node artifact itself and force the copy path.
	want := []byte("bundle")
	if err := os.WriteFile(node.configPath, want, 0o644); err != nil {
		t.Fatal(err)
	}
	node.Cfg.StartupConfig = node.configPath
	node.Cfg.EnforceStartupConfig = true

	if err := node.prepareStartupConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(node.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("artifact = %q, want %q", got, want)
	}
}

func TestPrepareECLIStartupOverlay(t *testing.T) {
	labDir := t.TempDir()
	source := filepath.Join(t.TempDir(), "olt.txt")
	want := []byte("[LT1]\nconfig\ncommit\n")
	if err := os.WriteFile(source, want, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &clabtypes.NodeConfig{
		ShortName:     "olt1",
		LabDir:        labDir,
		StartupConfig: source,
	}
	node := &lightOLT{}
	if err := node.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ResStartupConfig != node.configPath {
		t.Fatalf("saved config path = %q, want %q", cfg.ResStartupConfig, node.configPath)
	}
	if err := os.MkdirAll(node.configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := node.prepareStartupConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Env[startupConfigEnv]; got != "/clab/config/light-olt-startup.txt" {
		t.Fatalf("%s = %q", startupConfigEnv, got)
	}
	got, err := os.ReadFile(node.startupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("copied eCLI overlay = %q, want %q", got, want)
	}
}

func TestPrepareStartupConfigPreservesSavedBundle(t *testing.T) {
	newNode := func(t *testing.T) (*lightOLT, string) {
		t.Helper()
		source := filepath.Join(t.TempDir(), "requested.tgz")
		if err := os.WriteFile(source, []byte("requested"), 0o600); err != nil {
			t.Fatal(err)
		}
		node := &lightOLT{}
		if err := node.Init(&clabtypes.NodeConfig{
			ShortName:     "olt1",
			LabDir:        t.TempDir(),
			StartupConfig: source,
		}); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(node.configDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(node.configPath, []byte("saved"), 0o644); err != nil {
			t.Fatal(err)
		}
		return node, source
	}

	t.Run("saved bundle wins by default", func(t *testing.T) {
		node, _ := newNode(t)
		if err := node.prepareStartupConfig(context.Background()); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(node.configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "saved" {
			t.Fatalf("saved bundle was replaced: %q", got)
		}
	})

	t.Run("enforce replaces the saved bundle", func(t *testing.T) {
		node, _ := newNode(t)
		node.Cfg.EnforceStartupConfig = true
		if err := node.prepareStartupConfig(context.Background()); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(node.configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "requested" {
			t.Fatalf("enforced bundle = %q, want requested", got)
		}
	})
}

func TestPrepareECLIStartupOverlayUsesSavedBundleOnNextDeploy(t *testing.T) {
	labDir := t.TempDir()
	source := filepath.Join(t.TempDir(), "olt.txt")
	if err := os.WriteFile(source, []byte("[LT1]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &clabtypes.NodeConfig{
		ShortName:     "olt1",
		LabDir:        labDir,
		StartupConfig: source,
	}
	node := &lightOLT{}
	if err := node.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(node.configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node.configPath, []byte("saved"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := node.prepareStartupConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	if node.startupPath != node.configPath {
		t.Fatalf("startup path = %q, want saved bundle %q", node.startupPath, node.configPath)
	}
	if got := cfg.Env[startupConfigEnv]; got != "/clab/config/light-olt-startup.tgz" {
		t.Fatalf("%s = %q, want saved bundle", startupConfigEnv, got)
	}
	got, err := os.ReadFile(node.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "saved" {
		t.Fatalf("saved bundle was replaced: %q", got)
	}
}

func TestPreDeployHonoursCancelledContext(t *testing.T) {
	node := &lightOLT{}
	if err := node.Init(&clabtypes.NodeConfig{ShortName: "olt1", LabDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := node.PreDeploy(ctx, &clabnodes.PreDeployParams{TopologyName: "test"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PreDeploy() error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(node.configDir); statErr == nil {
		t.Fatal("config directory was created for a cancelled context")
	}
}

func TestSaveConfig(t *testing.T) {
	labDir := t.TempDir()
	cfg := &clabtypes.NodeConfig{
		ShortName: "olt",
		LongName:  "clab-test-olt",
		LabDir:    labDir,
	}
	node := &lightOLT{}
	if err := node.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(node.configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	runtime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	runtime.EXPECT().
		Exec(gomock.Any(), "clab-test-olt", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			wantCmd := []string{configBundleCommand, "export", "/clab/config/light-olt-startup.tgz"}
			if !reflect.DeepEqual(cmd.GetCmd(), wantCmd) {
				t.Fatalf("exec command = %#v, want %#v", cmd.GetCmd(), wantCmd)
			}
			if err := os.WriteFile(node.configPath, []byte("bundle"), 0o644); err != nil {
				t.Fatal(err)
			}
			return clabexec.NewExecResult(cmd), nil
		})
	node.WithRuntime(runtime)

	result, err := node.SaveConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.ConfigPath != node.configPath {
		t.Fatalf("SaveConfig result = %#v, want path %q", result, node.configPath)
	}
}

func TestSaveConfigFailsWhenExportDoesNotCreateTheBundle(t *testing.T) {
	cfg := &clabtypes.NodeConfig{
		ShortName: "olt",
		LongName:  "clab-test-olt",
		LabDir:    t.TempDir(),
	}
	node := &lightOLT{}
	if err := node.Init(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(node.configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	runtime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	runtime.EXPECT().
		Exec(gomock.Any(), "clab-test-olt", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			// The command reports success but writes nothing.
			return clabexec.NewExecResult(cmd), nil
		})
	node.WithRuntime(runtime)

	if _, err := node.SaveConfig(context.Background()); err == nil {
		t.Fatal("SaveConfig() succeeded without producing a bundle")
	} else if !strings.Contains(err.Error(), node.configPath) {
		t.Fatalf("SaveConfig() error = %v, want it to name %q", err, node.configPath)
	}
}

func TestInitPreservesOverrides(t *testing.T) {
	cfg := &clabtypes.NodeConfig{
		ShortName:     "olt1",
		ShmSize:       "2GiB",
		RestartPolicy: "on-failure",
		Healthcheck: &clabtypes.HealthcheckConfig{
			Test: []string{"CMD", "true"},
		},
	}
	node := &lightOLT{}

	if err := node.Init(cfg); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if cfg.ShmSize != "2GiB" {
		t.Fatalf("ShmSize override was replaced: %q", cfg.ShmSize)
	}
	if cfg.RestartPolicy != "on-failure" {
		t.Fatalf("RestartPolicy override was replaced: %q", cfg.RestartPolicy)
	}
	if got := cfg.Healthcheck.Test; !reflect.DeepEqual(got, []string{"CMD", "true"}) {
		t.Fatalf("Healthcheck override was replaced: %#v", got)
	}
}

func TestCheckInterfaceName(t *testing.T) {
	tests := map[string]struct {
		interfaces []string
		wantError  bool
	}{
		"single data interface":    {interfaces: []string{"eth1"}},
		"multiple data interfaces": {interfaces: []string{"eth1", "eth2", "eth10"}},
		"management interface":     {interfaces: []string{"eth0"}, wantError: true},
		"non-ethernet name":        {interfaces: []string{"port1"}, wantError: true},
		"leading zero":             {interfaces: []string{"eth01"}, wantError: true},
		"separator":                {interfaces: []string{"eth-1"}, wantError: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			node := &lightOLT{}
			if err := node.Init(&clabtypes.NodeConfig{ShortName: "olt1"}); err != nil {
				t.Fatalf("Init() error = %v", err)
			}

			for _, iface := range tc.interfaces {
				node.Endpoints = append(node.Endpoints, &clablinks.EndpointVeth{
					EndpointGeneric: clablinks.EndpointGeneric{IfaceName: iface},
				})
			}

			err := node.CheckInterfaceName()
			if tc.wantError && err == nil {
				t.Fatalf("CheckInterfaceName() accepted %#v", tc.interfaces)
			}
			if !tc.wantError && err != nil {
				t.Fatalf("CheckInterfaceName() rejected %#v: %v", tc.interfaces, err)
			}
		})
	}
}

func TestInterfaceAliases(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
		alias string
	}{
		"NT uplink":          {input: "1/2/1", want: "eth1", alias: "1/2/1"},
		"network port one":   {input: "1/1/1", want: "eth2", alias: "1/1/1"},
		"network port two":   {input: "1/1/2", want: "eth3", alias: "1/1/2"},
		"network port three": {input: "1/1/3", want: "eth4", alias: "1/1/3"},
		"network port four":  {input: "1/1/4", want: "eth5", alias: "1/1/4"},
		"native Linux name":  {input: "eth2", want: "eth2"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			node := &lightOLT{}
			if err := node.Init(&clabtypes.NodeConfig{ShortName: "olt1"}); err != nil {
				t.Fatal(err)
			}
			endpoint := &clablinks.EndpointVeth{
				EndpointGeneric: clablinks.EndpointGeneric{IfaceName: tc.input},
			}

			if err := node.AddEndpoint(endpoint); err != nil {
				t.Fatal(err)
			}
			if got := endpoint.GetIfaceName(); got != tc.want {
				t.Fatalf("interface name = %q, want %q", got, tc.want)
			}
			if got := endpoint.GetIfaceAlias(); got != tc.alias {
				t.Fatalf("interface alias = %q, want %q", got, tc.alias)
			}
			if err := node.CheckInterfaceName(); err != nil {
				t.Fatalf("mapped interface rejected: %v", err)
			}
		})
	}
}

func TestAddEndpointRejectsCollisions(t *testing.T) {
	tests := map[string]struct {
		names     []string
		wantNamed string
	}{
		"alias and native name": {names: []string{"1/1/1", "eth2"}, wantNamed: "1/1/1"},
		"native name and alias": {names: []string{"eth2", "1/1/1"}, wantNamed: "eth2"},
		"duplicate native name": {names: []string{"eth1", "eth1"}, wantNamed: "eth1"},
		"duplicate alias":       {names: []string{"1/2/1", "1/2/1"}, wantNamed: "1/2/1"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			node := &lightOLT{}
			if err := node.Init(&clabtypes.NodeConfig{ShortName: "olt1"}); err != nil {
				t.Fatal(err)
			}

			var err error
			for _, ifaceName := range tc.names {
				endpoint := &clablinks.EndpointVeth{
					EndpointGeneric: clablinks.EndpointGeneric{IfaceName: ifaceName},
				}
				if err = node.AddEndpoint(endpoint); err != nil {
					break
				}
			}

			if err == nil {
				t.Fatalf("AddEndpoint() accepted colliding interfaces %#v", tc.names)
			}
			// The error must name the configured port, not only the translated
			// Linux interface.
			if !strings.Contains(err.Error(), tc.wantNamed) {
				t.Fatalf("AddEndpoint() error = %v, want it to name %q", err, tc.wantNamed)
			}
		})
	}
}
