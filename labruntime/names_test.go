package labruntime

import (
	"strings"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: "r1", want: "r1"},
		{name: "R1", want: "r1"},
		{name: "PE_1", want: "pe-1"},
		{name: "Spine-01", want: "spine-01"},
		{name: "node one", want: "node-one"},
		{name: "7750-SR", want: "clab-7750-sr"},
		{name: "-leaf-", want: "leaf"},
		{name: "srv6+flexalgo", want: "srv6-flexalgo"},
		{name: "___", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SanitizeName(tt.name)
			if got != tt.want {
				t.Fatalf("SanitizeName(%q) = %q, want %q", tt.name, got, tt.want)
			}
			if again := SanitizeName(got); again != got {
				t.Fatalf("SanitizeName(%q) = %q, want the sanitized name to be stable", got, again)
			}
		})
	}
}

func TestSanitizeNameTruncatesToADNSLabel(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("Router", 20)

	got := SanitizeName(long)
	if len(got) != kubernetesNameMaxLen {
		t.Fatalf("len(SanitizeName(long)) = %d, want %d", len(got), kubernetesNameMaxLen)
	}
	if again := SanitizeName(got); again != got {
		t.Fatalf("SanitizeName(%q) = %q, want the sanitized name to be stable", got, again)
	}
	if other := SanitizeName(long + "X"); other == got {
		t.Fatal("truncated names of different nodes must not collapse onto one name")
	}
}

func TestSanitizeNodeNames(t *testing.T) {
	t.Parallel()

	renames, err := SanitizeNodeNames([]string{"R1", "R2", "client1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(renames) != 2 || renames["R1"] != "r1" || renames["R2"] != "r2" {
		t.Fatalf("renames = %v, want only R1 and R2 renamed", renames)
	}
}

func TestSanitizeNodeNamesRejectsCollidingNames(t *testing.T) {
	t.Parallel()

	_, err := SanitizeNodeNames([]string{"R1", "r1"})
	if err == nil || !strings.Contains(err.Error(), "both map onto") {
		t.Fatalf("SanitizeNodeNames() error = %v, want a collision error", err)
	}

	if _, err := SanitizeNodeNames([]string{"___"}); err == nil {
		t.Fatal("SanitizeNodeNames() error = nil, want an error for an unusable node name")
	}
}
