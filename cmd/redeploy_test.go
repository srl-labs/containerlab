package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRedeployRejectsRuntimeFlagsBeforeDestroy(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		flag    string
		options DeployOptions
	}{
		{"--no-topology-cr", DeployOptions{NoTopologyCR: true}},
		{"--image-pull-secret", DeployOptions{ImagePullSecret: "regcred"}},
		{"--no-persistence", DeployOptions{NoPersistence: true}},
	} {
		t.Run(tt.flag, func(t *testing.T) {
			o := &Options{Global: &GlobalOptions{Runtime: "docker"}, Deploy: &tt.options}
			// Destruction cannot run with the deliberately absent destroy options. The
			// validation error must be returned before that path is even entered.
			err := redeployFn(&cobra.Command{}, o)
			if err == nil || !strings.Contains(err.Error(), tt.flag+" is only supported") {
				t.Fatalf("redeploy validation error = %v", err)
			}
		})
	}
}
