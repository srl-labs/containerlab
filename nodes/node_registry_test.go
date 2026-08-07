package nodes

import "testing"

func TestNodeRegistryEntryPrivilegedByDefault(t *testing.T) {
	unprivilegedAttrs := NewNodeRegistryEntryAttributes(nil, nil, nil).
		WithPrivilegedByDefault(false)

	tests := []struct {
		name       string
		attributes *NodeRegistryEntryAttributes
		want       bool
	}{
		{name: "no attributes", want: true},
		{
			name:       "default attributes",
			attributes: NewNodeRegistryEntryAttributes(nil, nil, nil),
			want:       true,
		},
		{name: "unprivileged override", attributes: unprivilegedAttrs},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := newRegistryEntry(nil, nil, tt.attributes)
			if got := entry.PrivilegedByDefault(); got != tt.want {
				t.Fatalf("PrivilegedByDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
