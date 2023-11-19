package nodes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/srl-labs/containerlab/types"
)

func TestGenerateConfigs(t *testing.T) {
	tests := map[string]struct {
		node   *DefaultNode
		err    error
		exists bool
		out    string
	}{
		"suppress-true": {
			node: &DefaultNode{
				Cfg: &types.NodeConfig{
					SuppressStartupConfig: true,
					ShortName:             "suppress",
				},
			},
			err:    nil,
			exists: false,
			out:    "",
		},
		"suppress-false": {
			node: &DefaultNode{
				Cfg: &types.NodeConfig{
					SuppressStartupConfig: false,
					ShortName:             "configure",
				},
			},
			err:    nil,
			exists: true,
			out:    "foo",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			dstFolder := tt.TempDir()
			dstFile := filepath.Join(dstFolder, "config")
			err := tc.node.GenerateConfig(dstFile, "foo")
			if err != tc.err {
				t.Errorf("got %v, wanted %v", err, tc.err)
			}
			if tc.exists {
				cnt, err := os.ReadFile(dstFile)
				if err != nil {
					t.Fatal(err)
				}
				if string(cnt) != tc.out {
					t.Errorf("got %v, wanted %v", string(cnt), tc.out)
				}
			} else {
				if _, err := os.Stat(dstFile); err == nil {
					t.Errorf("config file created")
				}
			}
		})
	}
}
