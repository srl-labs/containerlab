package links

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestActivateLinkRequestSetsUpInDefaultMode(t *testing.T) {
	const index = 42

	msg := newActivateLinkMessage(index)

	if msg.Index != index {
		t.Fatalf("Index = %d, want %d", msg.Index, index)
	}
	if msg.Change != unix.IFF_UP {
		t.Fatalf("Change = %#x, want IFF_UP", msg.Change)
	}
	if msg.Flags&unix.IFF_UP == 0 {
		t.Fatal("IFF_UP is not set")
	}

	mode := newDefaultLinkModeAttr()
	if mode.Type != unix.IFLA_LINKMODE {
		t.Fatalf("link mode attribute type = %d, want IFLA_LINKMODE", mode.Type)
	}
	if len(mode.Data) != 1 || mode.Data[0] != 0 {
		t.Fatalf("link mode attribute data = %v, want IF_LINK_MODE_DEFAULT", mode.Data)
	}
}
