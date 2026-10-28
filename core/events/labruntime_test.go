package events

import (
	"testing"
	"time"
)

func TestLabRuntimeEventStreamRequestScope(t *testing.T) {
	t.Parallel()

	opts := Options{
		IncludeInitialState:   true,
		IncludeInterfaceStats: true,
		StatsInterval:         2 * time.Second,
	}

	all := labRuntimeEventStreamRequest("", opts)
	if all.Name != "" || !all.AllNamespaces {
		t.Fatalf("no lab = %+v, want every namespace", all)
	}
	if !all.IncludeInitialState || !all.IncludeInterfaceStats ||
		all.StatsInterval != 2*time.Second {
		t.Fatalf("options were not copied: %+v", all)
	}

	lab := labRuntimeEventStreamRequest("lab1", opts)
	if lab.Name != "lab1" || lab.AllNamespaces {
		t.Fatalf("lab = %+v, want the lab only", lab)
	}
}
