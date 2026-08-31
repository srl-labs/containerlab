package links

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestNewActivateLinkMessageClearsDormantWhileSettingUp(t *testing.T) {
	const index = 42

	msg := newActivateLinkMessage(index)

	if msg.Index != index {
		t.Fatalf("Index = %d, want %d", msg.Index, index)
	}
	if msg.Change != unix.IFF_UP|unix.IFF_DORMANT {
		t.Fatalf("Change = %#x, want IFF_UP|IFF_DORMANT", msg.Change)
	}
	if msg.Flags&unix.IFF_UP == 0 {
		t.Fatal("IFF_UP is not set")
	}
	if msg.Flags&unix.IFF_DORMANT != 0 {
		t.Fatal("IFF_DORMANT is set")
	}
}
