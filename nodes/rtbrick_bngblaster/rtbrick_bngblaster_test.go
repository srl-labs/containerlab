// Copyright 2026 RtBrick, Inc.
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package rtbrick_bngblaster

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestInit(t *testing.T) {
	customHealthcheck := &clabtypes.HealthcheckConfig{Test: []string{"CMD", "true"}}

	tests := map[string]struct {
		cfg  *clabtypes.NodeConfig
		want *clabtypes.NodeConfig
	}{
		"defaults without startup-config": {
			cfg: &clabtypes.NodeConfig{LabDir: "/lab/bbl"},
			want: &clabtypes.NodeConfig{
				LabDir:      "/lab/bbl",
				Image:       DefaultImage,
				Env:         map[string]string{},
				Sysctls:     map[string]string{sysctlDisableIPv6: "1"},
				Healthcheck: &defaultHealthcheck,
				Binds:       []string{"/lab/bbl:/var/bngblaster"},
			},
		},
		"startup-config enables autostart": {
			cfg: &clabtypes.NodeConfig{
				LabDir:        "/lab/bbl",
				StartupConfig: "bbl.json",
			},
			want: &clabtypes.NodeConfig{
				LabDir:           "/lab/bbl",
				StartupConfig:    "bbl.json",
				ResStartupConfig: "/lab/bbl/clab/config.json",
				Image:            DefaultImage,
				Env:              map[string]string{envAutostart: "clab"},
				Sysctls:          map[string]string{sysctlDisableIPv6: "1"},
				Healthcheck:      &defaultHealthcheck,
				Binds:            []string{"/lab/bbl:/var/bngblaster"},
			},
		},
		"suppressed startup-config does not autostart": {
			cfg: &clabtypes.NodeConfig{
				LabDir:                "/lab/bbl",
				StartupConfig:         "bbl.json",
				SuppressStartupConfig: true,
			},
			want: &clabtypes.NodeConfig{
				LabDir:                "/lab/bbl",
				StartupConfig:         "bbl.json",
				SuppressStartupConfig: true,
				ResStartupConfig:      "/lab/bbl/clab/config.json",
				Image:                 DefaultImage,
				Env:                   map[string]string{},
				Sysctls:               map[string]string{sysctlDisableIPv6: "1"},
				Healthcheck:           &defaultHealthcheck,
				Binds:                 []string{"/lab/bbl:/var/bngblaster"},
			},
		},
		"no sysctl with host network": {
			cfg: &clabtypes.NodeConfig{LabDir: "/lab/bbl", NetworkMode: "host"},
			want: &clabtypes.NodeConfig{
				LabDir:      "/lab/bbl",
				NetworkMode: "host",
				Image:       DefaultImage,
				Env:         map[string]string{},
				Healthcheck: &defaultHealthcheck,
				Binds:       []string{"/lab/bbl:/var/bngblaster"},
			},
		},
		"no sysctl with shared network namespace": {
			cfg: &clabtypes.NodeConfig{LabDir: "/lab/bbl", NetworkMode: "container:peer"},
			want: &clabtypes.NodeConfig{
				LabDir:      "/lab/bbl",
				NetworkMode: "container:peer",
				Image:       DefaultImage,
				Env:         map[string]string{},
				Healthcheck: &defaultHealthcheck,
				Binds:       []string{"/lab/bbl:/var/bngblaster"},
			},
		},
		"user settings win": {
			cfg: &clabtypes.NodeConfig{
				LabDir:        "/lab/bbl",
				StartupConfig: "bbl.json",
				Image:         "ghcr.io/rtbrick/bngblaster:dev",
				Env:           map[string]string{envAutostart: "test"},
				Sysctls:       map[string]string{sysctlDisableIPv6: "0"},
				Healthcheck:   customHealthcheck,
				Binds:         []string{"run.json:/var/bngblaster/clab/run.json:ro"},
			},
			want: &clabtypes.NodeConfig{
				LabDir:           "/lab/bbl",
				StartupConfig:    "bbl.json",
				ResStartupConfig: "/lab/bbl/clab/config.json",
				Image:            "ghcr.io/rtbrick/bngblaster:dev",
				Env:              map[string]string{envAutostart: "test"},
				Sysctls:          map[string]string{sysctlDisableIPv6: "0"},
				Healthcheck:      customHealthcheck,
				Binds: []string{
					"run.json:/var/bngblaster/clab/run.json:ro",
					"/lab/bbl:/var/bngblaster",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := new(bngblaster)
			if err := n.Init(tc.cfg); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, n.Cfg); diff != "" {
				t.Errorf("config mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeployWaitInterfaces(t *testing.T) {
	endpoints := []clablinks.Endpoint{
		&clablinks.EndpointVeth{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "eth1"}},
		&clablinks.EndpointVeth{EndpointGeneric: clablinks.EndpointGeneric{IfaceName: "eth2"}},
	}

	tests := map[string]struct {
		env  map[string]string
		want string
	}{
		"links counted with autostart": {env: map[string]string{envAutostart: "clab"}, want: "2"},
		"user value wins": {
			env:  map[string]string{envAutostart: "clab", envWaitInterfaces: "1"},
			want: "1",
		},
		"no wait without autostart":    {env: map[string]string{}, want: ""},
		"no wait with empty autostart": {env: map[string]string{envAutostart: ""}, want: ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
			n := new(bngblaster)
			if err := n.Init(
				&clabtypes.NodeConfig{Env: tc.env},
				clabnodes.WithRuntime(rt),
			); err != nil {
				t.Fatal(err)
			}
			n.Endpoints = endpoints

			rt.EXPECT().CreateContainer(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, cfg *clabtypes.NodeConfig) (string, error) {
					if got := cfg.Env[envWaitInterfaces]; got != tc.want {
						t.Errorf("%s = %q, want %q", envWaitInterfaces, got, tc.want)
					}
					return "id", nil
				})
			rt.EXPECT().StartContainer(gomock.Any(), "id", gomock.Any())

			if err := n.Deploy(context.Background(), &clabnodes.DeployParams{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPreDestroyStopsContainer(t *testing.T) {
	tests := map[string]struct {
		status   clabruntime.ContainerStatus
		wantStop bool
	}{
		"running container is stopped":       {status: clabruntime.Running, wantStop: true},
		"stopped container is left as it is": {status: clabruntime.Stopped, wantStop: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))
			n := new(bngblaster)
			cfg := &clabtypes.NodeConfig{LongName: "clab-lab-bbl"}
			if err := n.Init(cfg, clabnodes.WithRuntime(rt)); err != nil {
				t.Fatal(err)
			}

			rt.EXPECT().GetContainerStatus(gomock.Any(), "clab-lab-bbl").Return(tc.status)
			if tc.wantStop {
				// A failed stop is not an error, the container is removed anyway.
				rt.EXPECT().StopContainer(gomock.Any(), "clab-lab-bbl", gomock.Any()).
					Return(errors.New("stop failed"))
			}

			if err := n.PreDestroy(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckInterfaceName(t *testing.T) {
	tests := map[string]struct {
		ifaces      []string
		networkMode string
		wantErr     bool
	}{
		"data interfaces":           {ifaces: []string{"eth1", "eth2"}},
		"any interface name":        {ifaces: []string{"access1", "network1"}},
		"eth0 with management":      {ifaces: []string{"eth0"}, wantErr: true},
		"eth0 without management":   {ifaces: []string{"eth0"}, networkMode: "none"},
		"duplicate interface names": {ifaces: []string{"eth1", "eth1"}, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := new(bngblaster)
			if err := n.Init(
				&clabtypes.NodeConfig{ShortName: "bbl", NetworkMode: tc.networkMode},
			); err != nil {
				t.Fatal(err)
			}
			for _, iface := range tc.ifaces {
				n.Endpoints = append(
					n.Endpoints,
					&clablinks.EndpointVeth{
						EndpointGeneric: clablinks.EndpointGeneric{IfaceName: iface},
					},
				)
			}
			if err := n.CheckInterfaceName(); (err != nil) != tc.wantErr {
				t.Fatalf("CheckInterfaceName() error = %v, want error %v", err, tc.wantErr)
			}
		})
	}
}
