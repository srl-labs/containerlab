package events

import (
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

func TestLabRuntimeEventStreamRequestScope(t *testing.T) {
	t.Parallel()

	opts := Options{
		IncludeInitialState:   true,
		IncludeInterfaceStats: true,
		StatsInterval:         2 * time.Second,
	}

	all := labRuntimeEventStreamRequest("", clablabruntime.Identity{}, opts)
	if all.Name != "" || !all.AllNamespaces {
		t.Fatalf("no lab = %+v, want every namespace", all)
	}
	if !all.IncludeInitialState || !all.IncludeInterfaceStats ||
		all.StatsInterval != 2*time.Second {
		t.Fatalf("options were not copied: %+v", all)
	}

	identity := clablabruntime.Identity{User: "clab:alice", Groups: []string{"clab:users"}}
	lab := labRuntimeEventStreamRequest("lab1", identity, opts)
	if lab.Name != "lab1" || lab.AllNamespaces || !lab.Identity.Equal(identity) {
		t.Fatalf("lab = %+v, want the lab only", lab)
	}
}
