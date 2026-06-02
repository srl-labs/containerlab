package dependency_manager

import (
	"testing"
)

func Test_recursiveAcyclicityCheck(t *testing.T) {
	type args struct {
		dependencies map[string][]string
		i            int
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Single node - non cyclic",
			args: args{
				dependencies: map[string][]string{
					"node1": {},
				},
				i: 1,
			},
			want: true,
		},
		{
			name: "Multiple nodes",
			args: args{
				dependencies: map[string][]string{
					"node1": {"node2"},
					"node2": {"node3"},
					"node3": {"node4"},
					"node4": {"node5"},
					"node5": {"node6"},
					"node6": {"node7"},
					"node7": {},
				},
				i: 1,
			},
			want: true,
		},
		{
			name: "Multiple nodes - cyclic",
			args: args{
				dependencies: map[string][]string{
					"node1": {"node2"},
					"node2": {"node3"},
					"node3": {"node4"},
					"node4": {"node5"},
					"node5": {"node6"},
					"node6": {"node7"},
					"node7": {"node1"},
				},
				i: 1,
			},
			want: false,
		},
		{
			name: "Multiple nodes - cyclic",
			args: args{
				dependencies: map[string][]string{
					"node1": {},
					"node2": {"node1"},
					"node3": {"node2"},
					"node4": {"node3"},
					"node5": {"node2"},
					"node6": {"node7"},
					"node7": {"node1", "node2", "node3", "node5"},
				},
				i: 1,
			},
			want: true,
		},
		{
			name: "Multiple nodes - cyclic",
			args: args{
				dependencies: map[string][]string{
					"node1": {},
					"node2": {"node1"},
					"node3": {"node1", "node2"},
					"node4": {"node1", "node2", "node3"},
					"node5": {"node2"},
					"node6": {"node7"},
					"node7": {"node1", "node2", "node3", "node5"},
				},
				i: 1,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAcyclic(tt.args.dependencies, tt.args.i); got != tt.want {
				t.Errorf("recursiveAcyclicityCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}
