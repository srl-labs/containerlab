package types

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/srl-labs/containerlab/utils"
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
					Kind:       "srl",
					CPU:        1,
					Memory:     "1G",
					AutoRemove: utils.Pointer(true),
					DNS: &DNSConfig{
						Servers: []string{"1.1.1.1"},
						Search:  []string{"foo.com"},
						Options: []string{"someopt"},
					},
					Certificate: &CertificateConfig{
						Issue: utils.Pointer(true),
					},
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:       "srl",
				CPU:        1,
				Memory:     "1G",
				AutoRemove: utils.Pointer(true),
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: utils.Pointer(true),
				},
			},
		},
	},
	"node_kind": {
		input: &Topology{
			Kinds: map[string]*NodeDefinition{
				"srl": {
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
					AutoRemove: utils.Pointer(true),
					DNS: &DNSConfig{
						Servers: []string{"8.8.8.8"},
						Search:  []string{"bar.com"},
						Options: []string{"someotheropt"},
					},
					Certificate: &CertificateConfig{
						Issue: utils.Pointer(true),
					},
				},
			},
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Kind: "srl",
					Env: map[string]string{
						"env2": "notv2",
					},
					Labels: map[string]string{
						"label2": "notv2",
					},
					Memory:     "2G",
					AutoRemove: utils.Pointer(false),
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
				Kind:          "srl",
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
				AutoRemove: utils.Pointer(false),
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: utils.Pointer(true),
				},
			},
		},
	},
	"node_kind_default": {
		input: &Topology{
			Defaults: &NodeDefinition{
				Kind: "srl",
				User: "user1",
				CPU:  1,
				Binds: []string{
					"x:z",
					"m:n", // overriden by node
				},
			},
			Kinds: map[string]*NodeDefinition{
				"srl": {
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
				Kind:          "srl",
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
					Issue: utils.Pointer(false),
				},
			},
		},
	},
	"node_default": {
		input: &Topology{
			Defaults: &NodeDefinition{
				Kind:          "srl",
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
				Kind:          "srl",
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
				AutoRemove: utils.Pointer(false),
				DNS: &DNSConfig{
					Servers: []string{"1.1.1.1"},
					Search:  []string{"foo.com"},
					Options: []string{"someopt"},
				},
				Certificate: &CertificateConfig{
					Issue: utils.Pointer(false),
				},
			},
		},
	},
}

func TestGetNodeKind(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		kind := item.input.GetNodeKind("node1")
		if item.want["node1"].Kind != kind {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Kind)
			t.Errorf("item %q got %q", name, kind)
			t.Fail()
		}
	}
}

func TestGetNodeGroup(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		group := item.input.GetNodeGroup("node1")
		t.Logf("%q test item result: %v", name, group)
		if item.want["node1"].Group != group {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Group)
			t.Errorf("item %q got %q", name, group)
			t.Fail()
		}
	}
}

func TestGetNodeType(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		typ := item.input.GetNodeType("node1")
		t.Logf("%q test item result: %v", name, typ)
		if !cmp.Equal(item.want["node1"].Type, typ) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Type)
			t.Errorf("item %q got %q", name, typ)
			t.Fail()
		}
	}
}

func TestGetNodeConfig(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		config := item.input.GetNodeStartupConfig("node1")
		wantedConfig := item.want["node1"].StartupConfig
		t.Logf("%q test item result: %v", name, config)
		if !cmp.Equal(wantedConfig, config) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, wantedConfig)
			t.Errorf("item %q got %q", name, config)
			t.Fail()
		}
	}
}

func TestGetNodeImage(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		image := item.input.GetNodeImage("node1")
		t.Logf("%q test item result: %v", name, image)
		if item.want["node1"].Image != image {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Image)
			t.Errorf("item %q got %q", name, image)
			t.Fail()
		}
	}
}

func TestGetNodeLicense(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		lic := item.input.GetNodeLicense("node1")
		wantedLicense := item.want["node1"].License
		t.Logf("%q test item result: %v", name, lic)
		if !cmp.Equal(wantedLicense, lic) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, wantedLicense)
			t.Errorf("item %q got %q", name, lic)
			t.Fail()
		}
	}
}

func TestGetNodePosition(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		pos := item.input.GetNodePosition("node1")
		t.Logf("%q test item result: %v", name, pos)
		if !cmp.Equal(item.want["node1"].Position, pos) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Position)
			t.Errorf("item %q got %q", name, pos)
			t.Fail()
		}
	}
}

func TestGetNodeCmd(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		cmd := item.input.GetNodeCmd("node1")
		t.Logf("%q test item result: %v", name, cmd)
		if !cmp.Equal(item.want["node1"].Cmd, cmd) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Cmd)
			t.Errorf("item %q got %q", name, cmd)
			t.Fail()
		}
	}
}

func TestGetNodeExec(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		exec := item.input.GetNodeExec("node1")
		t.Logf("%q test item result: %v", name, exec)
		if !cmp.Equal(item.want["node1"].Exec, exec) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Exec)
			t.Errorf("item %q got %q", name, exec)
			t.Fail()
		}
	}
}

func TestGetNodeUser(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		user := item.input.GetNodeUser("node1")
		t.Logf("%q test item result: %v", name, user)
		if !cmp.Equal(item.want["node1"].User, user) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].User)
			t.Errorf("item %q got %q", name, user)
			t.Fail()
		}
	}
}

func TestGetNodeBinds(t *testing.T) {
	for _, item := range topologyTestSet {
		binds, _ := item.input.GetNodeBinds("node1")

		// sort the slices so we can compare them
		slices.Sort(binds)
		slices.Sort(item.want["node1"].Binds)

		if d := cmp.Diff(binds, item.want["node1"].Binds); d != "" {
			t.Fatalf("Binds resolve failed.\nGot: %q\nWant: %q\nDiff\n%s", binds, item.want["node1"].Binds, d)
		}
	}
}

func TestGetNodeEnv(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		envs := item.input.GetNodeEnv("node1")
		t.Logf("%q test item result: %v", name, envs)
		if !cmp.Equal(item.want["node1"].Env, envs) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Env)
			t.Errorf("item %q got %q", name, envs)
			t.Fail()
		}
	}
}

func TestGetNodeLabels(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		labels := item.input.GetNodeLabels("node1")
		t.Logf("%q test item result: %v", name, labels)
		if !cmp.Equal(item.want["node1"].Labels, labels) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Labels)
			t.Errorf("item %q got %q", name, labels)
			t.Fail()
		}
	}
}

func TestGetNodeAutoRemove(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		autoremove := item.input.GetNodeAutoRemove("node1")
		t.Logf("%q test item result: %v", name, autoremove)
		if item.want["node1"].AutoRemove != nil && *item.want["node1"].AutoRemove != autoremove {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %v", name, item.want["node1"].AutoRemove)
			t.Errorf("item %q got %v", name, autoremove)
			t.Fail()
		}
	}
}

func TestGetNodeDNS(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)

		dns := item.input.GetNodeDns("node1")

		t.Logf("%q test item result: %v", name, dns)

		if d := cmp.Diff(item.want["node1"].DNS, dns); d != "" {
			t.Fatalf("DNS config object doesn't match.\nGot: %+v\nWant: %+v\nDiff\n%s", dns, item.want["node1"].DNS, d)
		}
	}
}

func TestGetNodeCertificateConfig(t *testing.T) {
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)

		cert := item.input.GetCertificateConfig("node1")

		t.Logf("%q test item result: %v", name, cert)

		if d := cmp.Diff(item.want["node1"].Certificate, cert); d != "" {
			t.Fatalf("Certificate config objects don't match.\nGot: %+v\nWant: %+v\nDiff\n%s",
				cert, item.want["node1"].Certificate, d)
		}
	}
}
