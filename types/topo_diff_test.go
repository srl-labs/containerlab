package types

import "testing"

func TestTopologyDiffDefaultAction(t *testing.T) {
	tests := []struct {
		name string
		diff *TopologyDiff
		want TopologyDiffAction
	}{
		{name: "nil", want: TopologyDiffActionNone},
		{name: "empty", diff: &TopologyDiff{}, want: TopologyDiffActionNone},
		{name: "exec", diff: &TopologyDiff{Fields: []string{"Exec"}}, want: TopologyDiffActionRecreate},
		{name: "unknown", diff: &TopologyDiff{Fields: []string{"FutureField"}}, want: TopologyDiffActionRecreate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.DefaultAction(); got != tt.want {
				t.Fatalf("DefaultAction() = %q, want %q", got, tt.want)
			}
		})
	}
}
