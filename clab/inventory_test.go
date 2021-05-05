package clab

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGenerateAnsibleInventory(t *testing.T) {
	tests := map[string]struct {
		got  string
		want string
	}{
		"case1": {
			got: "test_data/topo1.yml",
			want: `all:
  children:
    srl:
      hosts:
        clab-topo1-node1:
          ansible_host: 172.100.100.11
        clab-topo1-node2:
          ansible_host: 172.100.100.12
`,
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

			var s strings.Builder
			err := c.generateAnsibleInventory(&s)
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(s.String(), tc.want) {
				t.Errorf("failed at '%s', expected\n%v, got\n%+v", name, tc.want, s.String())
			}
		})
	}
}
