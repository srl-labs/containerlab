package types

import "testing"

var topologyTestSet = map[string]struct {
	input *Topology
	want  map[string]*NodeDefinition
}{
	"node_only": {
		input: &Topology{
			Nodes: map[string]*NodeDefinition{
				"node1": {
					Kind: "srl",
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind: "srl",
			},
		},
	},
	"node_kind": {
		input: &Topology{
			Kinds: map[string]*NodeDefinition{
				"srl": {
					Group:    "grp1",
					Type:     "type1",
					Config:   "config.cfg",
					Image:    "image:latest",
					License:  "lic1.key",
					Position: "pos1",
					Cmd:      "runit",
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
				},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:     "srl",
				Group:    "grp1",
				Type:     "type1",
				Config:   "config.cfg",
				Image:    "image:latest",
				License:  "lic1.key",
				Position: "pos1",
				Cmd:      "runit",
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
			},
		},
	},
	"node_kind_default": {
		input: &Topology{
			Defaults: &NodeDefinition{
				Kind: "srl",
			},
			Kinds: map[string]*NodeDefinition{
				"srl": {
					Group:    "grp1",
					Type:     "type1",
					Config:   "config.cfg",
					Image:    "image:latest",
					License:  "lic1.key",
					Position: "pos1",
					Cmd:      "runit",
				},
			},
			Nodes: map[string]*NodeDefinition{
				"node1": {},
			},
		},
		want: map[string]*NodeDefinition{
			"node1": {
				Kind:     "srl",
				Group:    "grp1",
				Type:     "type1",
				Config:   "config.cfg",
				Image:    "image:latest",
				License:  "lic1.key",
				Position: "pos1",
				Cmd:      "runit",
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
