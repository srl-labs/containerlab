package types

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

func TestComponentMDAUnmarshal(t *testing.T) {
	mdaYaml := `
- slot: 1
  type: a
- slot: 2
  type: b
`

	var m MDAS
	if err := yaml.Unmarshal([]byte(mdaYaml), &m); err != nil {
		t.Fatalf("err unmarshalling mdas: %v", err)
	}

	if len(m) != 2 {
		t.Fatalf("got: %d mdas, want 2", len(m))
	}

	wantMDAS := MDAS{
		{
			Slot: 1,
			Type: "a",
		},
		{
			Slot: 2,
			Type: "b",
		},
	}

	if diff := cmp.Diff(wantMDAS, m); diff != "" {
		t.Fatalf("MDAS mismatch:\n%s", diff)
	}
}

func TestComponentMDAUnmarshalInvalidSlotZero(t *testing.T) {
	mdaYaml := `
- slot: 0
  type: foo
`

	var m MDAS

	if err := yaml.Unmarshal([]byte(mdaYaml), &m); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "invalid mda entry"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestComponentMDAUnmarshalInvalidSlotNonNumeric(t *testing.T) {
	mdaYaml := `
- slot: z
  type: foo
`

	var m MDAS

	if err := yaml.Unmarshal([]byte(mdaYaml), &m); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "cannot unmarshal"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestComponentMDAUnmarshalMissingType(t *testing.T) {
	mdaYaml := `
- slot: 1
`

	var m MDAS

	if err := yaml.Unmarshal([]byte(mdaYaml), &m); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "invalid mda entry"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestComponentMDAUnmarshalMissingSlot(t *testing.T) {
	mdaYaml := `
- type: abc
`

	var m MDAS

	if err := yaml.Unmarshal([]byte(mdaYaml), &m); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "invalid mda entry"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestComponentMDAUnmarshalDuplicateSlot(t *testing.T) {
	mdaYaml := `
- slot: 1
  type: a
- slot: 1
  type: b
`

	var m MDAS

	if err := yaml.Unmarshal([]byte(mdaYaml), &m); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "duplicate slot"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestXIOMSUnmarshalDuplicateSlot(t *testing.T) {
	xiomYaml := `
- slot: 1
  type: foo
- slot: 1
  type: foo
`

	var x XIOMS

	if err := yaml.Unmarshal([]byte(xiomYaml), &x); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "duplicate slot"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestXIOMNestedMDAUnmarshalDuplicateSlot(t *testing.T) {
	xiomYaml := `
- slot: 2
  type: foo
  mda:
    - slot: 1
      type: foo
    - slot: 1
      type: bar
`

	var x XIOMS

	if err := yaml.Unmarshal([]byte(xiomYaml), &x); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "invalid mda entry"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}

		if want := "duplicate slot"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestXIOMSUnmarshalInvalidSlotZero(t *testing.T) {
	xiomYaml := `
- slot: 0
  type: iom-s-1.5t
`

	var x XIOMS

	if err := yaml.Unmarshal([]byte(xiomYaml), &x); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "invalid xiom entry"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestXIOMSUnmarshalMissingType(t *testing.T) {
	xiomYaml := `
- slot: 1
`

	var x XIOMS

	if err := yaml.Unmarshal([]byte(xiomYaml), &x); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "invalid xiom entry"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestXIOMSUnmarshalMissingSlot(t *testing.T) {
	xiomYaml := `
- type: iom-s-1.5t
`

	var x XIOMS

	if err := yaml.Unmarshal([]byte(xiomYaml), &x); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "invalid xiom entry"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestXIOMSUnmarshalInvalidSlotNonNumeric(t *testing.T) {
	xiomYaml := `
- slot: z
  type: iom-s-1.5t
`

	var x XIOMS

	if err := yaml.Unmarshal([]byte(xiomYaml), &x); err == nil {
		t.Fatalf("expected error, got nil")
	} else {
		if want := "cannot unmarshal"; !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want contains %q", err.Error(), want)
		}
	}
}
