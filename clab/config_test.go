package clab

import (
	"path/filepath"
	"strings"
	"testing"
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

			nodeCfg := c.Config.Topology.Nodes["node1"]
			node := Node{}
			node.Kind = strings.ToLower(c.kindInitialization(&nodeCfg))

			lic, err := c.licenseInit(&nodeCfg, &node)
			if err != nil {
				t.Fatal(err)
			}
			if lic != tc.want {
				t.Fatalf("wanted '%s' got '%s'", tc.want, lic)
			}
		})
	}
}

func abspath(s string) string {
	p, _ := filepath.Abs(s)
	return p
}
