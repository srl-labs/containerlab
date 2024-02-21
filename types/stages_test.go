package types

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEquals(t *testing.T) {
	tests := []struct {
		name string
		w1   *WaitFor
		w2   *WaitFor
		want bool
	}{
		{
			name: "Equal nodes and states",
			w1: &WaitFor{
				Node:  "node1",
				Stage: "state1",
			},
			w2: &WaitFor{
				Node:  "node1",
				Stage: "state1",
			},
			want: true,
		},
		{
			name: "Different nodes and states",
			w1: &WaitFor{
				Node:  "node1",
				Stage: "state1",
			},
			w2: &WaitFor{
				Node:  "node2",
				Stage: "state2",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w1.Equals(tt.w2); got != tt.want {
				t.Errorf("WaitFor.Equals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		s       *Stages
		other   *Stages
		want    *Stages
		wantErr bool
	}{
		{
			name: "Merge non-nil stages",
			s: &Stages{
				Configure: &StageConfigure{StageBase: StageBase{
					WaitFor: WaitForList{
						&WaitFor{Node: "node1", Stage: "state1"},
					},
				}},
				// Create:      &StageCreate{StageBase: StageBase{Name: "Create1"}},
				// CreateLinks: &StageCreateLinks{StageBase: StageBase{Name: "CreateLinks1"}},
				Healthy: &StageHealthy{},
				// Exit:        &StageExit{StageBase: StageBase{Name: "Exit1"}},
			},
			other: &Stages{
				Configure: &StageConfigure{StageBase: StageBase{
					WaitFor: WaitForList{
						&WaitFor{Node: "node2", Stage: "state2"},
					},
				}},
				// Create:      &StageCreate{StageBase: StageBase{Name: "Create1"}},
				// CreateLinks: &StageCreateLinks{StageBase: StageBase{Name: "CreateLinks1"}},
				Healthy: &StageHealthy{StageBase: StageBase{
					WaitFor: WaitForList{
						&WaitFor{Node: "node1", Stage: "state1"},
					},
				}},
				// Exit:        &StageExit{StageBase: StageBase{Name: "Exit1"}},
			},
			want: &Stages{
				Configure: &StageConfigure{StageBase: StageBase{
					WaitFor: WaitForList{
						&WaitFor{Node: "node1", Stage: "state1"},
						&WaitFor{Node: "node2", Stage: "state2"},
					},
				}},
				// Create:      &Stage{StageBase: StageBase{Name: "Create1Create2"}},
				// CreateLinks: &Stage{StageBase: StageBase{Name: "CreateLinks1CreateLinks2"}},
				Healthy: &StageHealthy{StageBase: StageBase{
					WaitFor: WaitForList{
						&WaitFor{Node: "node1", Stage: "state1"},
					},
				}},
				// Exit:        &Stage{StageBase: StageBase{Name: "Exit1Exit2"}},
			},
			wantErr: false,
		},
		// Add more test cases as needed...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.s.Merge(tt.other)
			if (err != nil) != tt.wantErr {
				t.Errorf("Stages.Merge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.s, tt.want); diff != "" {
				t.Errorf("Stages.Merge() mismatch (+want -got):\n%s", diff)
			}
		})
	}
}
