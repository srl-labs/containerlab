package types

import (
	"reflect"
	"testing"
)

func TestSplitDockerAndTailscalePorts(t *testing.T) {
	docker, ts, err := SplitDockerAndTailscalePorts([]string{
		"80:8080",
		"8022:22/ts",
		"8574:57400/TS",
		"9000-9001:80-81/ts",
		"55555:43555/udp",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDocker := []string{"80:8080", "55555:43555/udp"}
	if !reflect.DeepEqual(docker, wantDocker) {
		t.Fatalf("docker = %#v, want %#v", docker, wantDocker)
	}
	wantTS := []TailscalePort{
		{Listen: 8022, Dest: 22},
		{Listen: 8574, Dest: 57400},
		{Listen: 9000, Dest: 80},
		{Listen: 9001, Dest: 81},
	}
	if !reflect.DeepEqual(ts, wantTS) {
		t.Fatalf("ts = %#v, want %#v", ts, wantTS)
	}
}

func TestSplitDockerAndTailscalePortsInvalid(t *testing.T) {
	for _, spec := range []string{
		"not-a-port/ts",
		"22/ts",
		"0:22/ts",
		"8022:0/ts",
		"127.0.0.1:8022:22/ts",
		"8000-8010:22/ts",
	} {
		if _, _, err := SplitDockerAndTailscalePorts([]string{spec}); err == nil {
			t.Errorf("%q: expected error", spec)
		}
	}
}

func TestGetNodePortMappings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ports   []string
		docker  bool
		ts      []TailscalePort
		wantErr bool
	}{
		{name: "empty"},
		{name: "Docker", ports: []string{"8080:80/tcp"}, docker: true},
		{
			name: "Tailscale range", ports: []string{"9000-9001:22-23/ts"},
			ts: []TailscalePort{{Listen: 9000, Dest: 22}, {Listen: 9001, Dest: 23}},
		},
		{
			name: "mixed", ports: []string{"8080:80/tcp", "8022:22/ts"}, docker: true,
			ts: []TailscalePort{{Listen: 8022, Dest: 22}},
		},
		{name: "invalid Docker", ports: []string{"invalid/tcp"}, wantErr: true},
		{name: "invalid Tailscale", ports: []string{"8022:0/ts"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			topo := NewTopology()
			// Inherited port mappings must take the same path as per-node mappings.
			topo.Defaults.Ports = tc.ports
			topo.Nodes["n1"] = &NodeDefinition{Kind: "linux"}
			ports, bindings, ts, err := topo.GetNodePortMappings("n1")
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !reflect.DeepEqual(ts, tc.ts) {
				t.Fatalf("Tailscale mappings = %v, want %v", ts, tc.ts)
			}
			if tc.docker {
				if _, ok := ports["80/tcp"]; !ok || len(ports) != 1 {
					t.Fatalf("Docker ports = %v", ports)
				}
				if b := bindings["80/tcp"]; len(b) != 1 || b[0].HostPort != "8080" {
					t.Fatalf("Docker bindings = %v", bindings)
				}
			} else if ports != nil || bindings != nil {
				t.Fatalf("unexpected Docker ports %v or bindings %v", ports, bindings)
			}
			oldPorts, oldBindings, err := topo.GetNodePorts("n1")
			if err != nil || !reflect.DeepEqual(oldPorts, ports) ||
				!reflect.DeepEqual(oldBindings, bindings) {
				t.Fatalf("GetNodePorts changed: %v, %v, %v", oldPorts, oldBindings, err)
			}
		})
	}
}

func TestTailscaleConfigProxy(t *testing.T) {
	if (&TailscaleConfig{AuthMode: "sso"}).Proxy() != true {
		t.Fatal("sso should be proxy")
	}
	if (&TailscaleConfig{AuthMode: "SSO"}).Proxy() != true {
		t.Fatal("SSO should be proxy")
	}
	if (&TailscaleConfig{AuthKey: "tskey"}).Proxy() {
		t.Fatal("auth-key only is not proxy")
	}
	var n *TailscaleConfig
	if n.Proxy() {
		t.Fatal("nil config is not proxy")
	}
}
