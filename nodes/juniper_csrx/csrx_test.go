// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package juniper_csrx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestKindNamesOnlyRegistersJuniperCSRX(t *testing.T) {
	want := []string{"juniper_csrx"}
	if !reflect.DeepEqual(kindNames, want) {
		t.Fatalf("kindNames = %#v, want %#v", kindNames, want)
	}
}

func TestInitSetsCSRXBindsAndDefaultEnv(t *testing.T) {
	labDir := t.TempDir()
	node := &csrx{}
	cfg := &clabtypes.NodeConfig{
		LabDir:    labDir,
		ShortName: "csrx1",
	}

	if err := node.Init(cfg); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	wantBinds := []string{
		filepath.Join(labDir, configDir) + ":/config",
		filepath.Join(labDir, "log") + ":/var/log",
		filepath.Join(labDir, configDir, sshdConfig) + ":/etc/ssh/sshd_config",
		filepath.Join(labDir, "csrx_password_config_file") + ":/var/local/csrx_password_config_file",
	}
	for _, want := range wantBinds {
		if !containsString(cfg.Binds, want) {
			t.Fatalf("missing bind %q in %#v", want, cfg.Binds)
		}
	}

	if got := cfg.Env["CSRX_JUNOS_CONFIG"]; got != containerConfig {
		t.Fatalf("CSRX_JUNOS_CONFIG = %q, want %q", got, containerConfig)
	}
}

func TestInitPreservesUserCSRXConfigEnv(t *testing.T) {
	cfg := &clabtypes.NodeConfig{
		LabDir:    t.TempDir(),
		ShortName: "csrx1",
		Env: map[string]string{
			"CSRX_JUNOS_CONFIG": "/custom/juniper.conf",
		},
	}
	node := &csrx{}

	if err := node.Init(cfg); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	if got := cfg.Env["CSRX_JUNOS_CONFIG"]; got != "/custom/juniper.conf" {
		t.Fatalf("CSRX_JUNOS_CONFIG = %q, want user-provided value", got)
	}
}

func TestCreateCSRXFilesGeneratesDefaultArtifacts(t *testing.T) {
	labDir := t.TempDir()
	node := newInitializedTestNode(t, &clabtypes.NodeConfig{
		LabDir:    labDir,
		ShortName: "csrx1",
	})

	if err := createCSRXFiles(node); err != nil {
		t.Fatalf("unexpected createCSRXFiles error: %v", err)
	}

	cfg := node.Config()
	if cfg.ResStartupConfig != csrxConfigPath(labDir) {
		t.Fatalf("ResStartupConfig = %q, want %q", cfg.ResStartupConfig, csrxConfigPath(labDir))
	}

	assertFileContains(t, csrxConfigPath(labDir), "root-authentication")
	assertFileExists(t, filepath.Join(labDir, configDir, sshdConfig))
	assertFileExists(t, filepath.Join(labDir, "csrx_password_config_file"))
	assertDirExists(t, filepath.Join(labDir, "log"))
}

func TestCreateCSRXFilesCopiesStartupConfigAndLicense(t *testing.T) {
	labDir := t.TempDir()

	startupConfig := filepath.Join(labDir, "startup.conf")
	if err := os.WriteFile(startupConfig, []byte("system { host-name csrx1; }\n"), 0o644); err != nil {
		t.Fatalf("failed to write startup config: %v", err)
	}

	license := filepath.Join(labDir, "license.lic")
	if err := os.WriteFile(license, []byte("license-content"), 0o644); err != nil {
		t.Fatalf("failed to write license: %v", err)
	}

	node := newInitializedTestNode(t, &clabtypes.NodeConfig{
		LabDir:        labDir,
		ShortName:     "csrx1",
		StartupConfig: startupConfig,
		License:       license,
	})

	if err := createCSRXFiles(node); err != nil {
		t.Fatalf("unexpected createCSRXFiles error: %v", err)
	}

	assertFileContains(t, csrxConfigPath(labDir), "host-name csrx1")
	assertFileContains(t, csrxLicensePath(labDir), "license-content")
}

func newInitializedTestNode(t *testing.T, cfg *clabtypes.NodeConfig) *csrx {
	t.Helper()

	node := &csrx{}
	if err := node.Init(cfg); err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}

	return node
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %q to exist: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected %q to be a file", path)
	}
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory %q to exist: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %q to be a directory", path)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %q: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("file %q content = %q, want substring %q", path, string(data), want)
	}
}
