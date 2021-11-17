package types

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

var topologyTestSet = map[string]struct {
	input *Topology
	want  map[string]*NodeDefinition
}{
	"node_only": {
		input: &Topology{
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Kind:   "srl",
					CPU:    1,
					Memory: "1G",
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:   "srl",
				CPU:    1,
				Memory: "1G",
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
					CPU:    1,
					Memory: "1G",
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
					Memory: "2G",
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
				CPU:    1,
				Memory: "2G",
			},
		},
	},
	"node_kind_default": {
		input: &Topology{
			Defaults: &NodeDefinition{
				Kind: "srl",
				User: "user1",
				CPU:  1,
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
				User:          "user1",
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
				Memory: "2G",
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
				CPU:    1,
				Memory: "1G",
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
		config, err := item.input.GetNodeStartupConfig("node1")
		if err != nil {
			t.Fatal(err)
		}
		wantedConfig, err := resolvePath(item.want["node1"].StartupConfig)
		if err != nil {
			t.Fatal(err)
		}
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
		lic, err := item.input.GetNodeLicense("node1")
		if err != nil {
			t.Fatal(err)
		}
		wantedLicense, err := resolvePath(item.want["node1"].License)
		if err != nil {
			t.Fatal(err)
		}
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
	for name, item := range topologyTestSet {
		t.Logf("%q test item", name)
		binds := item.input.GetNodeBinds("node1")
		t.Logf("%q test item result: %v", name, binds)
		if !cmp.Equal(item.want["node1"].Binds, binds) {
			t.Errorf("item %q failed", name)
			t.Errorf("item %q exp %q", name, item.want["node1"].Binds)
			t.Errorf("item %q got %q", name, binds)
			t.Fail()
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
