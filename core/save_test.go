package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	"go.uber.org/mock/gomock"
)

func TestSaveReturnsNodeErrors(t *testing.T) {
	errBoom := errors.New("boom")

	tests := []struct {
		name    string
		failing map[string]bool
		wantErr []string // substrings the joined error must contain; nil means no error
	}{
		{name: "all nodes save", failing: nil},
		{
			name:    "one node fails",
			failing: map[string]bool{"r2": true},
			wantErr: []string{`node "r2" save failed: boom`},
		},
		{
			name:    "every node fails",
			failing: map[string]bool{"r1": true, "r2": true, "r3": true},
			wantErr: []string{
				`node "r1" save failed: boom`,
				`node "r2" save failed: boom`,
				`node "r3" save failed: boom`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			c := &CLab{Nodes: map[string]clabnodes.Node{}}
			for _, name := range []string{"r1", "r2", "r3"} {
				n := clabmocksmocknodes.NewMockNode(ctrl)
				n.EXPECT().GetShortName().Return(name).AnyTimes()
				var err error
				if tt.failing[name] {
					err = errBoom
				}
				n.EXPECT().SaveConfig(gomock.Any()).Return(nil, err)
				c.Nodes[name] = n
			}

			err := c.Save(context.Background())

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("got error %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, errBoom) {
				t.Fatalf("got error %v, want it to wrap %v", err, errBoom)
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
			if got := strings.Count(err.Error(), "save failed"); got != len(tt.wantErr) {
				t.Errorf("error reports %d failures, want %d: %q", got, len(tt.wantErr), err)
			}
		})
	}
}
