// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	clabcert "github.com/srl-labs/containerlab/cert"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabruntimedocker "github.com/srl-labs/containerlab/runtime/docker"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestWaitForNodeDeployErrorPrecedence(t *testing.T) {
	nodeErr := errors.New("node failed")

	t.Run("parent cancellation takes precedence", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		nodeFailCh := make(chan error, 1)
		nodeFailCh <- nodeErr

		err := waitForNodeDeploy(ctx, &sync.WaitGroup{}, nodeFailCh)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waitForNodeDeploy() error = %v, want context.Canceled", err)
		}
		if errors.Is(err, nodeErr) {
			t.Fatalf("waitForNodeDeploy() returned node error after cancellation: %v", err)
		}
	})

	t.Run("internal failure preserves node error", func(t *testing.T) {
		nodeFailCh := make(chan error, 1)
		nodeFailCh <- nodeErr

		err := waitForNodeDeploy(context.Background(), &sync.WaitGroup{}, nodeFailCh)
		if !errors.Is(err, nodeErr) {
			t.Fatalf("waitForNodeDeploy() error = %v, want node error", err)
		}
	})
}

func TestCheckReconcileDeployOptionsRejectsManagementNetworkOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		option ClabOption
	}{
		{name: "network name", option: WithManagementNetworkName("other")},
		{name: "IPv4 subnet", option: WithManagementIpv4Subnet("10.0.0.0/24")},
		{name: "IPv6 subnet", option: WithManagementIpv6Subnet("2001:db8::/64")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := NewContainerLab(tt.option)
			if err != nil {
				t.Fatal(err)
			}
			c.Config.Name = "lab"

			err = c.checkReconcileDeployOptions(&DeployOptions{})
			if err == nil || !strings.Contains(err.Error(), "management network overrides") {
				t.Fatalf("expected management network override error, got %v", err)
			}
		})
	}
}

func TestCheckReconcileDeployOptionsAllowsTopologyManagementNetwork(t *testing.T) {
	t.Parallel()

	c, err := NewContainerLab()
	if err != nil {
		t.Fatal(err)
	}
	c.Config.Name = "lab"
	c.Config.Mgmt.Network = "from-topology"
	c.Config.Mgmt.IPv4Subnet = "10.0.0.0/24"
	c.Config.Mgmt.IPv6Subnet = "2001:db8::/64"

	if err := c.checkReconcileDeployOptions(&DeployOptions{}); err != nil {
		t.Fatalf("topology management settings must not be treated as overrides: %v", err)
	}
}

func TestWaitForApplyNetworkModeTargetNoTarget(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(&clabtypes.NodeConfig{ShortName: "n1"}).AnyTimes()

	c := &CLab{Nodes: map[string]clabnodes.Node{"n1": node}}

	if err := c.waitForApplyNetworkModeTarget(context.Background(), "n1"); err != nil {
		t.Fatalf("unexpected error for node without a network-mode target: %v", err)
	}
}

func TestWaitForApplyNetworkModeTargetInternalTargetAlreadyRunning(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	target := clabmocksmocknodes.NewMockNode(ctrl)
	sidecar := clabmocksmocknodes.NewMockNode(ctrl)
	mockRuntime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)

	target.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName: "target",
		LongName:  "clab-lab-target",
	}).AnyTimes()
	sidecar.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "sidecar",
		NetworkMode: "container:target",
	}).AnyTimes()
	mockRuntime.EXPECT().GetContainerStatus(gomock.Any(), "clab-lab-target").
		Return(clabruntime.Running)

	c := &CLab{
		Nodes: map[string]clabnodes.Node{"target": target, "sidecar": sidecar},
		Runtimes: map[string]clabruntime.ContainerRuntime{
			clabruntimedocker.RuntimeName: mockRuntime,
		},
		globalRuntimeName: clabruntimedocker.RuntimeName,
	}

	if err := c.waitForApplyNetworkModeTarget(context.Background(), "sidecar"); err != nil {
		t.Fatalf("unexpected error waiting for an already-running target: %v", err)
	}
}

func TestWaitForApplyNetworkModeTargetExternalTargetDelegates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	sidecar := clabmocksmocknodes.NewMockNode(ctrl)
	sidecar.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "sidecar",
		NetworkMode: "container:not-in-topology",
	}).AnyTimes()
	mockRuntime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)
	mockRuntime.EXPECT().GetContainerStatus(gomock.Any(), "not-in-topology").
		Return(clabruntime.Running)

	c := &CLab{
		Nodes: map[string]clabnodes.Node{"sidecar": sidecar},
		Runtimes: map[string]clabruntime.ContainerRuntime{
			clabruntimedocker.RuntimeName: mockRuntime,
		},
		globalRuntimeName: clabruntimedocker.RuntimeName,
	}

	if err := c.waitForApplyNetworkModeTarget(context.Background(), "sidecar"); err != nil {
		t.Fatalf(
			"expected external target not managed by this topology to be waited on like fresh "+
				"deploy does, got: %v",
			err,
		)
	}
}

// TestDeployNodesWaitsForNetworkModeTargetWithSingleWorker locks in the fix
// for a real deadlock: with a single worker and the dependent node queued
// before its network-mode target, waiting for the target inside the
// worker's own loop would block the only worker able to create that
// target. The wait must happen outside the worker pool.
func TestDeployNodesWaitsForNetworkModeTargetWithSingleWorker(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	target := clabmocksmocknodes.NewMockNode(ctrl)
	sidecar := clabmocksmocknodes.NewMockNode(ctrl)
	mockRuntime := clabmocksmockruntime.NewMockContainerRuntime(ctrl)

	var targetDeployed atomic.Bool

	target.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName: "target",
		LongName:  "clab-lab-target",
	}).AnyTimes()
	target.EXPECT().GetShortName().Return("target").AnyTimes()
	target.EXPECT().PreDeploy(gomock.Any(), gomock.Any()).Return(nil)
	target.EXPECT().Deploy(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *clabnodes.DeployParams) error {
			targetDeployed.Store(true)
			return nil
		},
	)
	target.EXPECT().UpdateConfigWithRuntimeInfo(gomock.Any()).Return(nil)

	sidecar.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName:   "sidecar",
		NetworkMode: "container:target",
	}).AnyTimes()
	sidecar.EXPECT().GetShortName().Return("sidecar").AnyTimes()
	sidecar.EXPECT().PreDeploy(gomock.Any(), gomock.Any()).DoAndReturn(
		func(context.Context, *clabnodes.PreDeployParams) error {
			if !targetDeployed.Load() {
				t.Error("sidecar was deployed before its network-mode target")
			}
			return nil
		},
	)
	sidecar.EXPECT().Deploy(gomock.Any(), gomock.Any()).Return(nil)
	sidecar.EXPECT().UpdateConfigWithRuntimeInfo(gomock.Any()).Return(nil)

	mockRuntime.EXPECT().GetContainerStatus(gomock.Any(), "clab-lab-target").DoAndReturn(
		func(context.Context, string) clabruntime.ContainerStatus {
			if targetDeployed.Load() {
				return clabruntime.Running
			}
			return clabruntime.NotFound
		},
	).AnyTimes()

	c := &CLab{
		Config: &Config{Name: "lab"},
		Nodes:  map[string]clabnodes.Node{"target": target, "sidecar": sidecar},
		Runtimes: map[string]clabruntime.ContainerRuntime{
			clabruntimedocker.RuntimeName: mockRuntime,
		},
		globalRuntimeName: clabruntimedocker.RuntimeName,
	}

	// The dependent is listed before its target on purpose, with exactly
	// one worker, so a naive in-worker wait would deadlock here.
	err := c.DeployNodes(context.Background(), []string{"sidecar", "target"}, 1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCertificateAuthoritySetupUsesEnvironmentCAWithoutSettings(t *testing.T) {
	tempDir := t.TempDir()
	topoFile := filepath.Join(tempDir, "topology.clab.yml")
	if err := os.WriteFile(topoFile, []byte("name: env-ca\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := NewContainerLab()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.TopoPaths.SetTopologyFilePath(topoFile); err != nil {
		t.Fatal(err)
	}
	c.Config.Name = "env-ca"
	if err := c.TopoPaths.SetLabDirByPrefix(c.Config.Name); err != nil {
		t.Fatal(err)
	}

	externalCA, err := clabcert.NewCA().GenerateCACert(&clabcert.CACSRInput{
		CommonName: "CA-FROM-ENV",
		Country:    "US",
		Expiry:     time.Hour,
		KeySize:    1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	certFile := filepath.Join(tempDir, "ca.pem")
	keyFile := filepath.Join(tempDir, "ca.key")
	if err := externalCA.Write(certFile, keyFile, ""); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAB_CA_CERT_FILE", certFile)
	t.Setenv("CLAB_CA_KEY_FILE", keyFile)

	if err := c.certificateAuthoritySetup(); err != nil {
		t.Fatal(err)
	}

	if got := c.TopoPaths.CaCertAbsFilename(); got != certFile {
		t.Fatalf("CA certificate path = %q, want %q", got, certFile)
	}
	loadedCA, err := c.Cert.LoadCaCert()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loadedCA.Cert, externalCA.Cert) {
		t.Fatal("certificateAuthoritySetup did not load the CA certificate from CLAB_CA_CERT_FILE")
	}
}
