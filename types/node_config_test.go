package types

import "testing"

func TestLabContainerName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		longName  string
		shortName string
		want      string
	}{
		"default prefix":    {"clab-lab-n1", "n1", "clab-lab-n1-netns"},
		"lab named as node": {"clab-srl02-srl02", "srl02", "clab-srl02-srl02-netns"},
		"lab name contains node name": {
			"clab-dc1-leaf1-leaf1",
			"leaf1",
			"clab-dc1-leaf1-leaf1-netns",
		},
		"node name in prefix": {"lab-x-lab", "lab", "lab-x-lab-netns"},
		"lab name prefix":     {"lab-n1", "n1", "lab-n1-netns"},
		"empty prefix":        {"n1", "n1", "n1-netns"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			n := &NodeConfig{LongName: tt.longName, ShortName: tt.shortName}
			if got := n.LabContainerName(tt.shortName + "-netns"); got != tt.want {
				t.Fatalf("LabContainerName() = %q, want %q", got, tt.want)
			}
		})
	}
}
