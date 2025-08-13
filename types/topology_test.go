package types

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	clabutils "github.com/srl-labs/containerlab/utils"
	"golang.org/x/exp/slices"
)

var topologyTestSet = map[string]struct {
	input *Topology
	want  map[string]*NodeDefinition
}{
	"node_only": {
		input: &Topology{
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Kind:       "nokia_srlinux",
					CPU:        1,
					Memory:     "1G",
					AutoRemove: clabutils.Pointer(true),
					DNS: &DNSConfig{
						Servers: []string{"1.1.1.1"},
						Search:  []string{"foo.com"},
						Options: []string{"someopt"},
					},
					Certificate: &CertificateConfig{
						Issue: clabutils.Pointer(true),
					},
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:       "nokia_srlinux",
				CPU:        1,
				Memory:     "1G",
				AutoRemove: clabutils.Pointer(true),
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: clabutils.Pointer(true),
				},
			},
		},
	},
	"node_kind": {
		input: &Topology{
			Kinds: map[string]*NodeDefinition{
				"nokia_srlinux": {
					Group:         "grp1",
					Type:          "type1",
					StartupConfig: "test_data/config.cfg",
					Image:         "image:latest",
					License:       "test_data/lic1.key",
					Position:      "pos1",
					Cmd:           "runit",
					Exec: []string{
						"bash test1.sh",
						"bash test2.sh",
					},
					User: "user1",
					Binds: []string{
						"a:b",
						"c:d",
					},
					Ports: []string{
						"80:8080",
					},
					Env: map[string]string{
						"env1": "v1",
						"env2": "v2",
					},
					Labels: map[string]string{
						"label1": "v1",
						"label2": "v2",
					},
					CPU:        1,
					Memory:     "1G",
					AutoRemove: clabutils.Pointer(true),
					DNS: &DNSConfig{
						Servers: []string{"8.8.8.8"},
						Search:  []string{"bar.com"},
						Options: []string{"someotheropt"},
					},
					Certificate: &CertificateConfig{
						Issue: clabutils.Pointer(true),
					},
				},
			},
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Kind: "nokia_srlinux",
					Env: map[string]string{
						"env2": "notv2",
					},
					Labels: map[string]string{
						"label2": "notv2",
					},
					Memory:     "2G",
					AutoRemove: clabutils.Pointer(false),
					DNS: &DNSConfig{
						Servers: []string{"1.1.1.1"},
						Search:  []string{"foo.com"},
						Options: []string{"someopt"},
					},
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:          "nokia_srlinux",
				Group:         "grp1",
				Type:          "type1",
				StartupConfig: "test_data/config.cfg",
				Image:         "image:latest",
				License:       "test_data/lic1.key",
				Position:      "pos1",
				Cmd:           "runit",
				Exec: []string{
					"bash test1.sh",
					"bash test2.sh",
				},
				User: "user1",
				Binds: []string{
					"a:b",
					"c:d",
				},
				Ports: []string{
					"80:8080",
				},
				Env: map[string]string{
					"env1": "v1",
					"env2": "notv2",
				},
				Labels: map[string]string{
					"label1": "v1",
					"label2": "notv2",
				},
				CPU:        1,
				Memory:     "2G",
				AutoRemove: clabutils.Pointer(false),
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: clabutils.Pointer(true),
				},
			},
		},
	},
	"node_kind_default": {
		input: &Topology{
			Defaults: &NodeDefinition{
				Kind: "nokia_srlinux",
				User: "user1",
				CPU:  1,
				Binds: []string{
					"x:z",
					"m:n", // overridden by node
				},
			},
			Kinds: map[string]*NodeDefinition{
				"nokia_srlinux": {
					Group:         "grp1",
					Type:          "type1",
					StartupConfig: "test_data/config.cfg",
					Image:         "image:latest",
					License:       "test_data/lic1.key",
					Position:      "pos1",
					Cmd:           "runit",
					Exec: []string{
						"bash test1.sh",
						"bash test2.sh",
					},
					Binds: []string{
						"a:b",
						"c:d",
					},
					Ports: []string{
						"80:8080",
					},
					Env: map[string]string{
						"env1": "v1",
						"env2": "v2",
					},
					Labels: map[string]string{
						"label1": "v1",
						"label2": "v2",
					},
					CPU:    2,
					Memory: "2G",
					DNS: &DNSConfig{
						Servers: []string{"1.1.1.1"},
						Search:  []string{"foo.com"},
						Options: []string{"someopt"},
					},
				},
			},
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Binds: []string{
						"e:f",
						"newm:n",
					},
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:          "nokia_srlinux",
				Group:         "grp1",
				Type:          "type1",
				StartupConfig: "test_data/config.cfg",
				Image:         "image:latest",
				License:       "test_data/lic1.key",
				Position:      "pos1",
				Cmd:           "runit",
				User:          "user1",
				Exec: []string{
					"bash test1.sh",
					"bash test2.sh",
				},
				Binds: []string{
					"e:f",
					"a:b",
					"c:d",
					"x:z",
					"newm:n",
				},
				Ports: []string{
					"80:8080",
				},
				Env: map[string]string{
					"env1": "v1",
					"env2": "v2",
				},
				Labels: map[string]string{
					"label1": "v1",
					"label2": "v2",
				},
				CPU:    1,
				Memory: "2G",
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: clabutils.Pointer(false),
				},
			},
		},
	},
	"node_default": {
		input: &Topology{
			Defaults: &NodeDefinition{
				Kind:          "nokia_srlinux",
				Group:         "grp1",
				Type:          "type1",
				StartupConfig: "test_data/config.cfg",
				Image:         "image:latest",
				License:       "test_data/lic1.key",
				Position:      "pos1",
				Cmd:           "runit",
				Exec: []string{
					"bash test1.sh",
					"bash test2.sh",
				},
				Binds: []string{
					"a:b",
					"c:d",
				},
				Ports: []string{
					"80:8080",
				},
				Env: map[string]string{
					"env1": "v1",
					"env2": "v2",
				},
				Labels: map[string]string{
					"label1": "v1",
					"label2": "v2",
				},
				CPU:    1,
				Memory: "1G",
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
			},
			Nodes: map[string]*NodeDefinition{
				"node1": {},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:          "nokia_srlinux",
				Group:         "grp1",
				Type:          "type1",
				StartupConfig: "test_data/config.cfg",
				Image:         "image:latest",
				License:       "test_data/lic1.key",
				Position:      "pos1",
				Cmd:           "runit",
				Exec: []string{
					"bash test1.sh",
					"bash test2.sh",
				},
				Binds: []string{
					"a:b",
					"c:d",
				},
				Ports: []string{
					"80:8080",
				},
				Env: map[string]string{
					"env1": "v1",
					"env2": "v2",
				},
				Labels: map[string]string{
					"label1": "v1",
					"label2": "v2",
				},
				CPU:        1,
				Memory:     "1G",
				AutoRemove: clabutils.Pointer(false),
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: clabutils.Pointer(false),
				},
			},
		},
	},
	"node_group": {
		input: &Topology{
			Groups: map[string]*NodeDefinition{
				"grp1": {
					Kind:          "nokia_srlinux",
					Type:          "type1",
					StartupConfig: "test_data/config.cfg",
					Image:         "image:latest",
					License:       "test_data/lic1.key",
					Position:      "pos1",
					Cmd:           "runit",
					Exec: []string{
						"bash test1.sh",
						"bash test2.sh",
					},
					User: "user1",
					Binds: []string{
						"a:b",
						"c:d",
					},
					Ports: []string{
						"80:8080",
					},
					Env: map[string]string{
						"env1": "v1",
						"env2": "v2",
					},
					Labels: map[string]string{
						"label1": "v1",
						"label2": "v2",
					},
					CPU:        1,
					Memory:     "1G",
					AutoRemove: clabutils.Pointer(true),
					DNS: &DNSConfig{
						Servers: []string{"8.8.8.8"},
						Search:  []string{"bar.com"},
						Options: []string{"someotheropt"},
					},
					Certificate: &CertificateConfig{
						Issue: clabutils.Pointer(true),
					},
				},
			},
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Group: "grp1",
					Env: map[string]string{
						"env2": "notv2",
					},
					Labels: map[string]string{
						"label2": "notv2",
					},
					Memory:     "2G",
					AutoRemove: clabutils.Pointer(false),
					DNS: &DNSConfig{
						Servers: []string{"1.1.1.1"},
						Search:  []string{"foo.com"},
						Options: []string{"someopt"},
					},
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:          "nokia_srlinux",
				Group:         "grp1",
				Type:          "type1",
				StartupConfig: "test_data/config.cfg",
				Image:         "image:latest",
				License:       "test_data/lic1.key",
				Position:      "pos1",
				Cmd:           "runit",
				Exec: []string{
					"bash test1.sh",
					"bash test2.sh",
				},
				User: "user1",
				Binds: []string{
					"a:b",
					"c:d",
				},
				Ports: []string{
					"80:8080",
				},
				Env: map[string]string{
					"env1": "v1",
					"env2": "notv2",
				},
				Labels: map[string]string{
					"label1": "v1",
					"label2": "notv2",
				},
				CPU:        1,
				Memory:     "2G",
				AutoRemove: clabutils.Pointer(false),
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: clabutils.Pointer(true),
				},
			},
		},
	},
	"node_group_kind": {
		input: &Topology{
			Kinds: map[string]*NodeDefinition{
				"nokia_srlinux": {
					User: "user0",
					CPU:  1,
					Binds: []string{
						"x:z",
						"m:n", // overridden by group
					},
				},
			},
			Groups: map[string]*NodeDefinition{
				"grp1": {
					Kind:          "nokia_srlinux",
					User:          "user1",
					Type:          "type1",
					StartupConfig: "test_data/config.cfg",
					Image:         "image:latest",
					License:       "test_data/lic1.key",
					Position:      "pos1",
					Cmd:           "runit",
					Exec: []string{
						"bash test1.sh",
						"bash test2.sh",
					},
					Binds: []string{
						"a:b",
						"c:d",
					},
					Ports: []string{
						"80:8080",
					},
					Env: map[string]string{
						"env1": "v1",
						"env2": "v2",
					},
					Labels: map[string]string{
						"label1": "v1",
						"label2": "v2",
					},
					CPU:    2,
					Memory: "2G",
					DNS: &DNSConfig{
						Servers: []string{"1.1.1.1"},
						Search:  []string{"foo.com"},
						Options: []string{"someopt"},
					},
				},
			},
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Group: "grp1",
					Binds: []string{
						"e:f",
						"newm:n",
					},
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:          "nokia_srlinux",
				Group:         "grp1",
				Type:          "type1",
				StartupConfig: "test_data/config.cfg",
				Image:         "image:latest",
				License:       "test_data/lic1.key",
				Position:      "pos1",
				Cmd:           "runit",
				User:          "user1",
				Exec: []string{
					"bash test1.sh",
					"bash test2.sh",
				},
				Binds: []string{
					"e:f",
					"a:b",
					"c:d",
					"x:z",
					"newm:n",
				},
				Ports: []string{
					"80:8080",
				},
				Env: map[string]string{
					"env1": "v1",
					"env2": "v2",
				},
				Labels: map[string]string{
					"label1": "v1",
					"label2": "v2",
				},
				CPU:    1,
				Memory: "2G",
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: clabutils.Pointer(false),
				},
			},
		},
	},
	"node_group_default": {
		input: &Topology{
			Defaults: &NodeDefinition{
				Kind: "linux", // overridden by group
				User: "user1",
				CPU:  1,
				Binds: []string{
					"x:z",
					"m:n", // overridden by node
				},
			},
			Groups: map[string]*NodeDefinition{
				"grp1": {
					Kind:          "nokia_srlinux",
					Type:          "type1",
					StartupConfig: "test_data/config.cfg",
					Image:         "image:latest",
					License:       "test_data/lic1.key",
					Position:      "pos1",
					Cmd:           "runit",
					Exec: []string{
						"bash test1.sh",
						"bash test2.sh",
					},
					Binds: []string{
						"a:b",
						"c:d",
					},
					Ports: []string{
						"80:8080",
					},
					Env: map[string]string{
						"env1": "v1",
						"env2": "v2",
					},
					Labels: map[string]string{
						"label1": "v1",
						"label2": "v2",
					},
					CPU:    2,
					Memory: "2G",
					DNS: &DNSConfig{
						Servers: []string{"1.1.1.1"},
						Search:  []string{"foo.com"},
						Options: []string{"someopt"},
					},
				},
			},
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Group: "grp1",
					Binds: []string{
						"e:f",
						"newm:n",
					},
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:          "nokia_srlinux",
				Group:         "grp1",
				Type:          "type1",
				StartupConfig: "test_data/config.cfg",
				Image:         "image:latest",
				License:       "test_data/lic1.key",
				Position:      "pos1",
				Cmd:           "runit",
				User:          "user1",
				Exec: []string{
					"bash test1.sh",
					"bash test2.sh",
				},
				Binds: []string{
					"e:f",
					"a:b",
					"c:d",
					"x:z",
					"newm:n",
				},
				Ports: []string{
					"80:8080",
				},
				Env: map[string]string{
					"env1": "v1",
					"env2": "v2",
				},
				Labels: map[string]string{
					"label1": "v1",
					"label2": "v2",
				},
				CPU:    1,
				Memory: "2G",
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: clabutils.Pointer(false),
				},
			},
		},
	},
}

func TestGetNodeKind(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		kind := item.input.GetNodeKind("node1")
		if diff := cmp.Diff(item.want["node1"].Kind, kind); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeGroup(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		group := item.input.GetNodeGroup("node1")
		t.Logf("%q test item result: %v", name, group)
		if diff := cmp.Diff(item.want["node1"].Group, group); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeType(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		typ := item.input.GetNodeType("node1")
		t.Logf("%q test item result: %v", name, typ)
		if diff := cmp.Diff(item.want["node1"].Type, typ); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeConfig(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		config := item.input.GetNodeStartupConfig("node1")
		wantedConfig := item.want["node1"].StartupConfig
		t.Logf("%q test item result: %v", name, config)
		if diff := cmp.Diff(wantedConfig, config); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeImage(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		image := item.input.GetNodeImage("node1")
		t.Logf("%q test item result: %v", name, image)
		if diff := cmp.Diff(item.want["node1"].Image, image); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeLicense(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		lic := item.input.GetNodeLicense("node1")
		wantedLicense := item.want["node1"].License
		t.Logf("%q test item result: %v", name, lic)
		if diff := cmp.Diff(wantedLicense, lic); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodePosition(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		pos := item.input.GetNodePosition("node1")
		t.Logf("%q test item result: %v", name, pos)
		if diff := cmp.Diff(item.want["node1"].Position, pos); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeCmd(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		cmd := item.input.GetNodeCmd("node1")
		t.Logf("%q test item result: %v", name, cmd)
		if diff := cmp.Diff(item.want["node1"].Cmd, cmd); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeExec(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		exec := item.input.GetNodeExec("node1")
		t.Logf("%q test item result: %v", name, exec)
		if diff := cmp.Diff(item.want["node1"].Exec, exec); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeUser(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		user := item.input.GetNodeUser("node1")
		t.Logf("%q test item result: %v", name, user)
		if diff := cmp.Diff(item.want["node1"].User, user); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeBinds(t *testing.T) {
	for _, item := range topologyTestSet {
		binds, _ := item.input.GetNodeBinds("node1")

		// sort the slices so we can compare them
		slices.Sort(binds)
		slices.Sort(item.want["node1"].Binds)

		if diff := cmp.Diff(item.want["node1"].Binds, binds); diff != "" {
			t.Fatalf("Binds resolve failed.\nGot: %q\nWant: %q\nDiff\n%s", binds, item.want["node1"].Binds, diff)
		}
	}
}

func TestGetNodeEnv(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		envs := item.input.GetNodeEnv("node1")
		t.Logf("%q test item result: %v", name, envs)
		if diff := cmp.Diff(item.want["node1"].Env, envs); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeLabels(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		labels := item.input.GetNodeLabels("node1")
		t.Logf("%q test item result: %v", name, labels)
		if diff := cmp.Diff(item.want["node1"].Labels, labels); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeAutoRemove(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		autoremove := item.input.GetNodeAutoRemove("node1")
		t.Logf("%q test item result: %v", name, autoremove)
		want := false
		if item.want["node1"].AutoRemove != nil {
			want = *item.want["node1"].AutoRemove
		}
		if diff := cmp.Diff(want, autoremove); diff != "" {
			t.Errorf("item %q failed: (-want +got)\n%s", name, diff)
		}
	}
}

func TestGetNodeDNS(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)

		dns := item.input.GetNodeDns("node1")

		t.Logf("%q test item result: %v", name, dns)

		if diff := cmp.Diff(item.want["node1"].DNS, dns); diff != "" {
			t.Fatalf("DNS config object doesn't match.\nGot: %+v\nWant: %+v\nDiff\n%s", dns, item.want["node1"].DNS, diff)
		}
	}
}

func TestGetNodeCertificateConfig(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)

		cert := item.input.GetCertificateConfig("node1")

		t.Logf("%q test item result: %v", name, cert)

		if diff := cmp.Diff(item.want["node1"].Certificate, cert); diff != "" {
			t.Fatalf("Certificate config objects don't match.\nGot: %+v\nWant: %+v\nDiff\n%s",
				cert, item.want["node1"].Certificate, diff)
		}
	}
}
