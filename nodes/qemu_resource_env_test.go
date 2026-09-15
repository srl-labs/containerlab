package nodes_test

import (
	"strings"
	"testing"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	cisco_xrd_vrouter "github.com/srl-labs/containerlab/nodes/cisco_xrd_vrouter"
	f5_bigipve "github.com/srl-labs/containerlab/nodes/f5_bigipve"
	fortinet_fortigate "github.com/srl-labs/containerlab/nodes/fortinet_fortigate"
	huawei_vrp "github.com/srl-labs/containerlab/nodes/huawei_vrp"
	vr_cat9kv "github.com/srl-labs/containerlab/nodes/vr_cat9kv"
	vr_pan "github.com/srl-labs/containerlab/nodes/vr_pan"
	vr_xrv9k "github.com/srl-labs/containerlab/nodes/vr_xrv9k"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestVMQEMUResourceEnv(t *testing.T) {
	registry := clabnodes.NewNodeRegistry()
	vr_xrv9k.Register(registry)
	vr_cat9kv.Register(registry)
	cisco_xrd_vrouter.Register(registry)
	huawei_vrp.Register(registry)
	fortinet_fortigate.Register(registry)
	vr_pan.Register(registry)
	f5_bigipve.Register(registry)

	tests := []struct {
		name      string
		kind      string
		env       map[string]string
		wantSMP   string
		wantMem   string
		wantCPU   string
		wantNoCLI bool
	}{
		{
			name:      "xrv9k defaults",
			kind:      "cisco_xrv9k",
			wantSMP:   "2",
			wantMem:   "16384",
			wantNoCLI: true,
		},
		{
			name:      "cat9kv defaults",
			kind:      "cisco_cat9kv",
			wantSMP:   "4",
			wantMem:   "18432",
			wantNoCLI: true,
		},
		{
			name:      "xrd vrouter defaults",
			kind:      "cisco_xrd_vrouter",
			wantSMP:   "4",
			wantMem:   "10240",
			wantNoCLI: true,
		},
		{
			name:    "huawei defaults",
			kind:    "huawei_vrp",
			wantSMP: "2",
			wantMem: "2048",
		},
		{
			name:    "fortigate defaults",
			kind:    "fortinet_fortigate",
			wantSMP: "2",
			wantMem: "2048",
		},
		{
			name:    "pan defaults",
			kind:    "paloalto_panos",
			wantSMP: "2",
			wantMem: "6144",
			wantCPU: "qemu64",
		},
		{
			name:    "f5 defaults",
			kind:    "f5_bigip-ve",
			wantSMP: "4",
			wantMem: "8192",
			wantCPU: "host",
		},
		{
			name: "user overrides",
			kind: "cisco_xrv9k",
			env: map[string]string{
				"QEMU_SMP":    "6",
				"QEMU_MEMORY": "24576",
			},
			wantSMP:   "6",
			wantMem:   "24576",
			wantNoCLI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := registry.NewNodeOfKind(tt.kind)
			if err != nil {
				t.Fatalf("failed to create %s: %v", tt.kind, err)
			}

			cfg := &clabtypes.NodeConfig{
				ShortName: "node1",
				LabDir:    t.TempDir(),
				Env:       tt.env,
				Credentials: clabtypes.NodeCredentials{
					Username: "user",
					Password: "password",
				},
			}
			err = node.Init(
				cfg,
				clabnodes.WithMgmtNet(&clabtypes.MgmtNet{
					IPv4Subnet: "172.20.20.0/24",
					IPv6Subnet: "2001:db8::/64",
				}),
			)
			if err != nil {
				t.Fatalf("failed to initialize %s: %v", tt.kind, err)
			}

			if got := cfg.Env["QEMU_SMP"]; got != tt.wantSMP {
				t.Errorf("QEMU_SMP = %q, want %q", got, tt.wantSMP)
			}
			if got := cfg.Env["QEMU_MEMORY"]; got != tt.wantMem {
				t.Errorf("QEMU_MEMORY = %q, want %q", got, tt.wantMem)
			}
			if tt.wantCPU != "" && cfg.Env["QEMU_CPU"] != tt.wantCPU {
				t.Errorf("QEMU_CPU = %q, want %q", cfg.Env["QEMU_CPU"], tt.wantCPU)
			}
			if _, ok := cfg.Env["VCPU"]; ok {
				t.Error("legacy VCPU environment variable is still configured")
			}
			if _, ok := cfg.Env["RAM"]; ok {
				t.Error("legacy RAM environment variable is still configured")
			}
			if tt.wantNoCLI && strings.Contains(cfg.Cmd, "--vcpu") {
				t.Errorf("legacy --vcpu argument is still configured in %q", cfg.Cmd)
			}
			if tt.wantNoCLI && strings.Contains(cfg.Cmd, "--ram") {
				t.Errorf("legacy --ram argument is still configured in %q", cfg.Cmd)
			}
		})
	}
}
