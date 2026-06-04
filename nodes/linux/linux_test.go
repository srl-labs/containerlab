package linux

import (
	"testing"

	clabnodes "github.com/srl-labs/containerlab/nodes"
)

func TestLinuxLinkApplyMode(t *testing.T) {
	if got := (&linux{}).LinkApplyMode(); got != clabnodes.LinkApplyModeLive {
		t.Fatalf("LinkApplyMode() = %q, want %q", got, clabnodes.LinkApplyModeLive)
	}
}
