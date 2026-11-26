package schemas

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestTopology(t *testing.T) {
	want, err := os.ReadFile("clab.schema.json")
	if err != nil {
		t.Fatal(err)
	}

	got := Topology()
	if !bytes.Equal(got, want) {
		t.Fatal("Topology() differs from clab.schema.json")
	}
	if !json.Valid(got) {
		t.Fatal("Topology() is not valid JSON")
	}

	// The caller gets its own copy, a change to it doesn't reach the next caller
	got[0] = 'x'
	if !bytes.Equal(Topology(), want) {
		t.Fatal("changing the returned schema changed the embedded one")
	}
}
