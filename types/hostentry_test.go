package types

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHostEntry_MethodChaining(t *testing.T) {
	// Test the fluent API pattern used throughout the codebase, e.g.:
	// NewHostEntry(...).SetDescription(...).SetContainerID(...)
	tests := []struct {
		name        string
		ip          string
		hostname    string
		ipversion   IpVersion
		description string
		containerID string
		wantEntry   *HostEntry
		wantString  string
	}{
		{
			name:        "full_chain_with_all_fields",
			ip:          "192.168.1.10",
			hostname:    "clab-test-node1",
			ipversion:   IpVersionV4,
			description: "Kind: srl",
			containerID: "abc123def456789ghijkl",
			wantEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "clab-test-node1",
				ipversion:   IpVersionV4,
				containerId: "abc123def456789ghijkl",
				description: "Kind: srl",
			},
			wantString: "192.168.1.10\tclab-test-node1 abc123def456\t# Kind: srl",
		},
		{
			name:        "chain_without_container_id",
			ip:          "192.168.1.11",
			hostname:    "clab-test-node2",
			ipversion:   IpVersionV4,
			description: "Kind: linux",
			containerID: "",
			wantEntry: &HostEntry{
				ip:          "192.168.1.11",
				name:        "clab-test-node2",
				ipversion:   IpVersionV4,
				containerId: "",
				description: "Kind: linux",
			},
			wantString: "192.168.1.11\tclab-test-node2\t# Kind: linux",
		},
		{
			name:        "chain_short_container_id_ignored",
			ip:          "10.0.0.5",
			hostname:    "clab-test-node3",
			ipversion:   IpVersionV4,
			description: "Kind: vr-sros",
			containerID: "short",
			wantEntry: &HostEntry{
				ip:          "10.0.0.5",
				name:        "clab-test-node3",
				ipversion:   IpVersionV4,
				containerId: "short",
				description: "Kind: vr-sros",
			},
			wantString: "10.0.0.5\tclab-test-node3\t# Kind: vr-sros",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the fluent API pattern
			got := NewHostEntry(tt.ip, tt.hostname, tt.ipversion).
				SetDescription(tt.description).
				SetContainerID(tt.containerID)

			if diff := cmp.Diff(tt.wantEntry, got, cmp.AllowUnexported(HostEntry{})); diff != "" {
				t.Fatalf("entry mismatch (-want +got):\n%s", diff)
			}

			gotString := got.ToHostEntryString()
			if diff := cmp.Diff(tt.wantString, gotString); diff != "" {
				t.Fatalf("string mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewHostEntry(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		hostname  string
		ipversion IpVersion
		want      *HostEntry
	}{
		{
			name:      "ipv4_entry",
			ip:        "192.168.1.10",
			hostname:  "test.example.com",
			ipversion: IpVersionV4,
			want: &HostEntry{
				ip:        "192.168.1.10",
				name:      "test.example.com",
				ipversion: IpVersionV4,
			},
		},
		{
			name:      "ipv6_entry",
			ip:        "2001:db8::1",
			hostname:  "ipv6.example.com",
			ipversion: IpVersionV6,
			want: &HostEntry{
				ip:        "2001:db8::1",
				name:      "ipv6.example.com",
				ipversion: IpVersionV6,
			},
		},
		{
			name:      "empty_values",
			ip:        "",
			hostname:  "",
			ipversion: IpVersionAny,
			want: &HostEntry{
				ip:        "",
				name:      "",
				ipversion: IpVersionAny,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewHostEntry(tt.ip, tt.hostname, tt.ipversion)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(HostEntry{})); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHostEntry_SetContainerID(t *testing.T) {
	tests := []struct {
		name        string
		hostEntry   *HostEntry
		containerID string
		want        *HostEntry
	}{
		{
			name: "set_container_id",
			hostEntry: &HostEntry{
				ip:        "192.168.1.10",
				name:      "test.example.com",
				ipversion: IpVersionV4,
			},
			containerID: "abc123def456789",
			want: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "abc123def456789",
			},
		},
		{
			name: "set_short_container_id",
			hostEntry: &HostEntry{
				ip:        "192.168.1.10",
				name:      "test.example.com",
				ipversion: IpVersionV4,
			},
			containerID: "abc123",
			want: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "abc123",
			},
		},
		{
			name: "set_empty_container_id",
			hostEntry: &HostEntry{
				ip:        "192.168.1.10",
				name:      "test.example.com",
				ipversion: IpVersionV4,
			},
			containerID: "",
			want: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "",
			},
		},
		{
			name: "overwrite_container_id",
			hostEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "old123container",
			},
			containerID: "new456container",
			want: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "new456container",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hostEntry.SetContainerID(tt.containerID)

			// Should return the same instance (fluent API)
			if got != tt.hostEntry {
				t.Fatalf("expected SetContainerID to return the same instance")
			}

			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(HostEntry{})); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHostEntry_SetDescription(t *testing.T) {
	tests := []struct {
		name        string
		hostEntry   *HostEntry
		description string
		want        *HostEntry
	}{
		{
			name: "set_description",
			hostEntry: &HostEntry{
				ip:        "192.168.1.10",
				name:      "test.example.com",
				ipversion: IpVersionV4,
			},
			description: "Test server",
			want: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				description: "Test server",
			},
		},
		{
			name: "empty_description",
			hostEntry: &HostEntry{
				ip:        "192.168.1.10",
				name:      "test.example.com",
				ipversion: IpVersionV4,
			},
			description: "",
			want: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				description: "",
			},
		},
		{
			name: "overwrite_description",
			hostEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				description: "Old description",
			},
			description: "New description",
			want: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				description: "New description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hostEntry.SetDescription(tt.description)

			// Should return the same instance
			if got != tt.hostEntry {
				t.Fatalf("expected SetDescription to return the same instance")
			}

			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(HostEntry{})); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHostEntry_ToHostEntryString(t *testing.T) {
	tests := []struct {
		name      string
		hostEntry *HostEntry
		want      string
	}{
		{
			name: "with_description",
			hostEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				description: "Test server",
			},
			want: "192.168.1.10\ttest.example.com\t# Test server",
		},
		{
			name: "without_description",
			hostEntry: &HostEntry{
				ip:        "192.168.1.10",
				name:      "test.example.com",
				ipversion: IpVersionV4,
			},
			want: "192.168.1.10\ttest.example.com",
		},
		{
			name: "ipv6_with_description",
			hostEntry: &HostEntry{
				ip:          "2001:db8::1",
				name:        "ipv6.example.com",
				ipversion:   IpVersionV6,
				description: "IPv6 server",
			},
			want: "2001:db8::1\tipv6.example.com\t# IPv6 server",
		},
		{
			name: "empty_description",
			hostEntry: &HostEntry{
				ip:          "10.0.0.1",
				name:        "host",
				ipversion:   IpVersionV4,
				description: "",
			},
			want: "10.0.0.1\thost",
		},
		{
			name: "with_container_id_full_length",
			hostEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "abc123def456789ghijkl",
			},
			want: "192.168.1.10\ttest.example.com abc123def456",
		},
		{
			name: "with_container_id_exact_12_chars",
			hostEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "abc123def456",
			},
			want: "192.168.1.10\ttest.example.com abc123def456",
		},
		{
			name: "with_container_id_less_than_12_chars",
			hostEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "abc123",
			},
			want: "192.168.1.10\ttest.example.com",
		},
		{
			name: "with_container_id_and_description",
			hostEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "abc123def456789ghijkl",
				description: "Test server",
			},
			want: "192.168.1.10\ttest.example.com abc123def456\t# Test server",
		},
		{
			name: "with_empty_container_id",
			hostEntry: &HostEntry{
				ip:          "192.168.1.10",
				name:        "test.example.com",
				ipversion:   IpVersionV4,
				containerId: "",
				description: "Test server",
			},
			want: "192.168.1.10\ttest.example.com\t# Test server",
		},
		{
			name: "ipv6_with_container_id_and_description",
			hostEntry: &HostEntry{
				ip:          "2001:db8::1",
				name:        "ipv6.example.com",
				ipversion:   IpVersionV6,
				containerId: "container123456",
				description: "IPv6 server",
			},
			want: "2001:db8::1\tipv6.example.com container123\t# IPv6 server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hostEntry.ToHostEntryString()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHostEntries_ToHostsConfig(t *testing.T) {
	entries := HostEntries{
		&HostEntry{
			ip:          "192.168.1.10",
			name:        "test1.example.com",
			ipversion:   IpVersionV4,
			description: "Test server 1",
		},
		&HostEntry{
			ip:        "192.168.1.11",
			name:      "test2.example.com",
			ipversion: IpVersionV4,
		},
		&HostEntry{
			ip:          "2001:db8::1",
			name:        "ipv6.example.com",
			ipversion:   IpVersionV6,
			description: "IPv6 server",
		},
	}

	entriesWithContainerID := HostEntries{
		&HostEntry{
			ip:          "192.168.1.10",
			name:        "test1.example.com",
			ipversion:   IpVersionV4,
			containerId: "abc123def456789",
			description: "Test server 1",
		},
		&HostEntry{
			ip:          "192.168.1.11",
			name:        "test2.example.com",
			ipversion:   IpVersionV4,
			containerId: "xyz789abc012345",
		},
		&HostEntry{
			ip:          "2001:db8::1",
			name:        "ipv6.example.com",
			ipversion:   IpVersionV6,
			containerId: "ipv6cont12345",
			description: "IPv6 server",
		},
	}

	tests := []struct {
		name      string
		entries   HostEntries
		ipversion IpVersion
		want      string
	}{
		{
			name:      "all_entries",
			entries:   entries,
			ipversion: IpVersionAny,
			want: "192.168.1.10\ttest1.example.com\t# Test server 1\n" +
				"192.168.1.11\ttest2.example.com\n" +
				"2001:db8::1\tipv6.example.com\t# IPv6 server\n",
		},
		{
			name:      "ipv4_only",
			entries:   entries,
			ipversion: IpVersionV4,
			want: "192.168.1.10\ttest1.example.com\t# Test server 1\n" +
				"192.168.1.11\ttest2.example.com\n",
		},
		{
			name:      "ipv6_only",
			entries:   entries,
			ipversion: IpVersionV6,
			want:      "2001:db8::1\tipv6.example.com\t# IPv6 server\n",
		},
		{
			name:      "empty_entries",
			entries:   HostEntries{},
			ipversion: IpVersionAny,
			want:      "",
		},
		{
			name:      "all_entries_with_container_id",
			entries:   entriesWithContainerID,
			ipversion: IpVersionAny,
			want: "192.168.1.10\ttest1.example.com abc123def456\t# Test server 1\n" +
				"192.168.1.11\ttest2.example.com xyz789abc012\n" +
				"2001:db8::1\tipv6.example.com ipv6cont1234\t# IPv6 server\n",
		},
		{
			name:      "ipv4_only_with_container_id",
			entries:   entriesWithContainerID,
			ipversion: IpVersionV4,
			want: "192.168.1.10\ttest1.example.com abc123def456\t# Test server 1\n" +
				"192.168.1.11\ttest2.example.com xyz789abc012\n",
		},
		{
			name:      "ipv6_only_with_container_id",
			entries:   entriesWithContainerID,
			ipversion: IpVersionV6,
			want:      "2001:db8::1\tipv6.example.com ipv6cont1234\t# IPv6 server\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entries.ToHostsConfig(tt.ipversion)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHostEntries_Merge(t *testing.T) {
	tests := []struct {
		name     string
		original HostEntries
		other    HostEntries
		want     HostEntries
	}{
		{
			name: "merge_entries",
			original: HostEntries{
				&HostEntry{ip: "192.168.1.10", name: "host1", ipversion: IpVersionV4},
			},
			other: HostEntries{
				&HostEntry{ip: "192.168.1.11", name: "host2", ipversion: IpVersionV4},
				&HostEntry{ip: "2001:db8::1", name: "host3", ipversion: IpVersionV6},
			},
			want: HostEntries{
				&HostEntry{ip: "192.168.1.10", name: "host1", ipversion: IpVersionV4},
				&HostEntry{ip: "192.168.1.11", name: "host2", ipversion: IpVersionV4},
				&HostEntry{ip: "2001:db8::1", name: "host3", ipversion: IpVersionV6},
			},
		},
		{
			name:     "merge_into_empty",
			original: HostEntries{},
			other: HostEntries{
				&HostEntry{ip: "10.0.0.1", name: "test", ipversion: IpVersionV4},
			},
			want: HostEntries{
				&HostEntry{ip: "10.0.0.1", name: "test", ipversion: IpVersionV4},
			},
		},
		{
			name: "merge_empty",
			original: HostEntries{
				&HostEntry{ip: "192.168.1.10", name: "host1", ipversion: IpVersionV4},
			},
			other: HostEntries{},
			want: HostEntries{
				&HostEntry{ip: "192.168.1.10", name: "host1", ipversion: IpVersionV4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of original for testing
			original := make(HostEntries, len(tt.original))
			copy(original, tt.original)

			original.Merge(tt.other)

			if diff := cmp.Diff(tt.want, original, cmp.AllowUnexported(HostEntry{})); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
