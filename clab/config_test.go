package clab

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLicenseInit(t *testing.T) {
	tests := map[string]struct {
		got  string
		want string
	}{
		"node_license": {
			got:  "test_data/topo1.yml",
			want: "node1.lic",
		},
		"kind_license": {
			got:  "test_data/topo2.yml",
			want: "kind.lic",
		},
		"default_license": {
			got:  "test_data/topo3.yml",
			want: "default.lic",
		},
		"kind_overwrite": {
			got:  "test_data/topo4.yml",
			want: "node1.lic",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := []ClabOption{
				WithTopoFile(tc.got),
			}
			c := NewContainerLab(opts...)
			if err := c.ParseTopology(); err != nil {
				t.Fatal(err)
			}

			if filepath.Base(c.Nodes["node1"].License) != tc.want {
				t.Fatalf("wanted '%s' got '%s'", tc.want, c.Nodes["node1"].License)
			}
		})
	}
}

func TestBindsInit(t *testing.T) {
	tests := map[string]struct {
		got  string
		want []string
	}{
		"node_sing_bind": {
			got:  "test_data/topo1.yml",
			want: []string{"test_data/node1.lic:/dst"},
		},
		"node_many_binds": {
			got:  "test_data/topo2.yml",
			want: []string{"test_data/node1.lic:/dst1", "test_data/kind.lic:/dst2"},
		},
		"kind_binds": {
			got:  "test_data/topo5.yml",
			want: []string{"test_data/kind.lic:/dst"},
		},
		"default_binds": {
			got:  "test_data/topo3.yml",
			want: []string{"test_data/default.lic:/dst"},
		},
		"node_binds_override": {
			got:  "test_data/topo4.yml",
			want: []string{"test_data/node1.lic:/dst"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := []ClabOption{
				WithTopoFile(tc.got),
			}
			c := NewContainerLab(opts...)
			if err := c.ParseTopology(); err != nil {
				t.Fatal(err)
			}

			nodeCfg := c.Config.Topology.Nodes["node1"]
			node := Node{}
			node.Kind = strings.ToLower(c.kindInitialization(&nodeCfg))

			binds := c.bindsInit(&nodeCfg)
			// resolve wanted paths as the binds paths are resolved as part of the c.ParseTopology
			err := resolveBindPaths(tc.want)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(binds, tc.want) {
				t.Fatalf("wanted %q got %q", tc.want, binds)
			}
		})
	}
}

func TestTypeInit(t *testing.T) {
	tests := map[string]struct {
		got  string
		node string
		want string
	}{
		"undefined_type_returns_default": {
			got:  "test_data/topo1.yml",
			node: "node2",
			want: "ixr6",
		},
		"node_type_override_kind_type": {
			got:  "test_data/topo2.yml",
			node: "node2",
			want: "ixr10",
		},
		"node_inherits_kind_type": {
			got:  "test_data/topo2.yml",
			node: "node1",
			want: "ixrd2",
		},
		"node_inherits_default_type": {
			got:  "test_data/topo3.yml",
			node: "node2",
			want: "ixrd2",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := []ClabOption{
				WithTopoFile(tc.got),
			}
			c := NewContainerLab(opts...)
			if err := c.ParseTopology(); err != nil {
				t.Fatal(err)
			}

			// nodeCfg := c.Config.Topology.Nodes[tc.node]
			// node := Node{}
			// node.Kind = strings.ToLower(c.kindInitialization(&nodeCfg))

			// ntype := c.typeInit(&nodeCfg, node.Kind)
			if !reflect.DeepEqual(c.Nodes[tc.node].NodeType, tc.want) {
				t.Fatalf("wanted %q got %q", tc.want, c.Nodes[tc.node].NodeType)
			}
		})
	}
}

func TestEnvInit(t *testing.T) {
	tests := map[string]struct {
		got  string
		node string
		want map[string]string
	}{
		"env_defined_at_node_level": {
			got:  "test_data/topo1.yml",
			node: "node1",
			want: map[string]string{
				"env1": "val1",
				"env2": "val2",
			},
		},
		"env_defined_at_kind_level": {
			got:  "test_data/topo2.yml",
			node: "node2",
			want: map[string]string{
				"env1": "val1",
			},
		},
		"env_defined_at_defaults_level": {
			got:  "test_data/topo3.yml",
			node: "node1",
			want: map[string]string{
				"env1": "val1",
			},
		},
		"env_defined_at_node_and_kind_and_default_level": {
			got:  "test_data/topo4.yml",
			node: "node1",
			want: map[string]string{
				"env1": "node",
				"env2": "kind",
				"env3": "global",
				"env4": "kind",
				"env5": "node",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := []ClabOption{
				WithTopoFile(tc.got),
			}
			c := NewContainerLab(opts...)
			if err := c.ParseTopology(); err != nil {
				t.Fatal(err)
			}

			nodeCfg := c.Config.Topology.Nodes[tc.node]
			kind := strings.ToLower(c.kindInitialization(&nodeCfg))
			env := c.envInit(&nodeCfg, kind)
			if !reflect.DeepEqual(env, tc.want) {
				t.Fatalf("wanted %q got %q", tc.want, env)
			}
		})
	}
}

func TestUserInit(t *testing.T) {
	tests := map[string]struct {
		got  string
		node string
		want string
	}{
		"user_defined_at_node_level": {
			got:  "test_data/topo1.yml",
			node: "node2",
			want: "custom",
		},
		"user_defined_at_kind_level": {
			got:  "test_data/topo2.yml",
			node: "node2",
			want: "customkind",
		},
		"user_defined_at_defaults_level": {
			got:  "test_data/topo3.yml",
			node: "node1",
			want: "customglobal",
		},
		"user_defined_at_node_and_kind_and_default_level": {
			got:  "test_data/topo4.yml",
			node: "node1",
			want: "customnode",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := []ClabOption{
				WithTopoFile(tc.got),
			}
			c := NewContainerLab(opts...)
			if err := c.ParseTopology(); err != nil {
				t.Fatal(err)
			}

			nodeCfg := c.Config.Topology.Nodes[tc.node]
			kind := strings.ToLower(c.kindInitialization(&nodeCfg))
			user := c.userInit(&nodeCfg, kind)
			if user != tc.want {
				t.Fatalf("wanted %q got %q", tc.want, user)
			}
		})
	}
}

func TestVerifyLinks(t *testing.T) {
	tests := map[string]struct {
		got  string
		want string
	}{
		"two_duplicated_links": {
			got:  "test_data/topo6.yml",
			want: "endpoints [\"lin1:eth1\" \"lin2:eth2\"] appeared more than once in the links section of the topology file",
		},
		"no_duplicated_links": {
			got:  "test_data/topo1.yml",
			want: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := []ClabOption{
				WithTopoFile(tc.got),
			}
			c := NewContainerLab(opts...)

			err := c.verifyLinks()
			if err != nil && err.Error() != tc.want {
				t.Fatalf("wanted %q got %q", tc.want, err.Error())
			}
			if err == nil && tc.want != "" {
				t.Fatalf("wanted %q got %q", tc.want, err.Error())
			}
		})
	}

}

func TestLablesInit(t *testing.T) {
	tests := map[string]struct {
		got  string
		node string
		want map[string]string
	}{
		"only_default_labels": {
			got:  "test_data/topo1.yml",
			node: "node1",
			want: map[string]string{
				"containerlab":      "topo1",
				"clab-node-kind":    "srl",
				"clab-node-type":    "ixr6",
				"clab-node-group":   "",
				"clab-node-lab-dir": "./clab-topo1/node1",
				"clab-topo-file":    "./test_data/topo1.yml",
			},
		},
		"custom_node_label": {
			got:  "test_data/topo1.yml",
			node: "node2",
			want: map[string]string{
				"containerlab":      "topo1",
				"clab-node-kind":    "srl",
				"clab-node-type":    "ixr6",
				"clab-node-group":   "",
				"clab-node-lab-dir": "./clab-topo1/node2",
				"clab-topo-file":    "./test_data/topo1.yml",
				"node-label":        "value",
			},
		},
		"custom_kind_label": {
			got:  "test_data/topo2.yml",
			node: "node1",
			want: map[string]string{
				"containerlab":      "topo2",
				"clab-node-kind":    "srl",
				"clab-node-type":    "ixrd2",
				"clab-node-group":   "",
				"clab-node-lab-dir": "./clab-topo2/node1",
				"clab-topo-file":    "./test_data/topo2.yml",
				"kind-label":        "value",
			},
		},
		"custom_default_label": {
			got:  "test_data/topo3.yml",
			node: "node2",
			want: map[string]string{
				"containerlab":      "topo3",
				"clab-node-kind":    "srl",
				"clab-node-type":    "ixrd2",
				"clab-node-group":   "",
				"clab-node-lab-dir": "./clab-topo3/node2",
				"clab-topo-file":    "./test_data/topo3.yml",
				"default-label":     "value",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			opts := []ClabOption{
				WithTopoFile(tc.got),
			}
			c := NewContainerLab(opts...)
			if err := c.ParseTopology(); err != nil {
				t.Fatal(err)
			}

			tc.want["clab-node-lab-dir"], _ = resolvePath(tc.want["clab-node-lab-dir"])
			tc.want["clab-topo-file"], _ = resolvePath(tc.want["clab-topo-file"])

			labels := c.Nodes[tc.node].Labels

			if !cmp.Equal(labels, tc.want) {
				t.Errorf("failed at '%s', expected\n%v, got\n%+v", name, tc.want, labels)
			}
		})
	}
}
