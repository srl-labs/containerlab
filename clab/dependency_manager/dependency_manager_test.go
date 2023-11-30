package dependency_manager

import (
	"sort"
	"strings"
	"testing"

	"github.com/srl-labs/containerlab/types"
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

func Test_defaultDependencyManager_String(t *testing.T) {
	type fields struct {
		nodes        map[string]*dependencyNode
		dependencies []struct {
			depender string
			dependee string
			waitFor  types.WaitForPhase
		}
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test one",
			fields: fields{
				nodes: map[string]*dependencyNode{
					"node1": newDependencyNode("node1"),
					"node2": newDependencyNode("node2"),
				},
				dependencies: []struct {
					depender string
					dependee string
					waitFor  types.WaitForPhase
				}{{
					depender: "node1",
					dependee: "node2",
					waitFor:  types.WaitForCreate,
				},
				},
			},
			want: `node1 -> [  ]
node2 -> [ node1 ]`,
		},
		{
			name: "test two",
			fields: fields{
				nodes: map[string]*dependencyNode{
					"node1": newDependencyNode("node1"),
					"node2": newDependencyNode("node2"),
					"node3": newDependencyNode("node3"),
					"node4": newDependencyNode("node4"),
				},
				dependencies: []struct {
					depender string
					dependee string
					waitFor  types.WaitForPhase
				}{
					{
						depender: "node1",
						dependee: "node2",
						waitFor:  types.WaitForCreate,
					},
					{
						depender: "node1",
						dependee: "node3",
						waitFor:  types.WaitForCreate,
					},
					{
						depender: "node3",
						dependee: "node4",
						waitFor:  types.WaitForCreate,
					},
				},
			},
			want: `node1 -> [  ]
node2 -> [ node1 ]
node3 -> [ node1 ]
node4 -> [ node3 ]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := &defaultDependencyManager{
				nodes: tt.fields.nodes,
			}
			for _, deps := range tt.fields.dependencies {
				dm.AddDependency(deps.depender, types.WaitForCreate, deps.dependee, types.WaitForCreate)
			}
			lines := strings.Split(dm.String(), "\n")
			sort.Strings(lines)
			sorted_result := strings.Join(lines, "\n")

			if sorted_result != tt.want {
				t.Errorf("defaultDependencyManager.String() = %v, want %v", sorted_result, tt.want)
			}
		})
	}
}
