package linux

import (
	"context"
	"testing"

	clabnodes "github.com/srl-labs/containerlab/nodes"
)

func TestLinuxLinkApplyMode(t *testing.T) {
	if got := (&linux{}).LinkApplyMode(context.Background()); got != clabnodes.LinkApplyModeLive {
		t.Fatalf("LinkApplyMode() = %q, want %q", got, clabnodes.LinkApplyModeLive)
	}
}
