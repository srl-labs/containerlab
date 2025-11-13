package utils

import (
	"net"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGenerateIPv6ULASubnet(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "generate_valid_ula",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateIPv6ULASubnet()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check the format: fd00:xxxx:xxxx:xxxx::/64
			pattern := `^fd00:[0-9a-f]{4}:[0-9a-f]{4}:[0-9a-f]{4}::/64$`
			matched, err := regexp.MatchString(pattern, got)
			if err != nil {
				t.Fatalf("regex error: %v", err)
			}
			if !matched {
				t.Fatalf("generated ULA subnet %q does not match expected pattern %q", got, pattern)
			}

			// Verify it's a valid IPv6 network
			_, _, err = net.ParseCIDR(got)
			if err != nil {
				t.Fatalf("generated subnet %q is not a valid IPv6 CIDR: %v", got, err)
			}

			// Verify it starts with fd00:
			if !strings.HasPrefix(got, "fd00:") {
				t.Fatalf("generated subnet %q does not start with 'fd00:'", got)
			}

			// Verify it ends with :/64
			if !strings.HasSuffix(got, ":/64") {
				t.Fatalf("generated subnet %q does not end with ':/64'", got)
			}
		})
	}
}

func TestGenerateIPv6ULASubnet_Uniqueness(t *testing.T) {
	// Generate multiple ULA subnets and verify they are different
	const numTests = 10
	subnets := make(map[string]bool)

	for i := 0; i < numTests; i++ {
		subnet, err := GenerateIPv6ULASubnet()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}

		if subnets[subnet] {
			t.Fatalf("generated duplicate subnet: %q", subnet)
		}
		subnets[subnet] = true
	}

	if len(subnets) != numTests {
		t.Fatalf("expected %d unique subnets, got %d", numTests, len(subnets))
	}
}

func TestCIDRToDDN(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   string
	}{
		{
			name:   "cidr_8",
			length: 8,
			want:   "255.0.0.0",
		},
		{
			name:   "cidr_16",
			length: 16,
			want:   "255.255.0.0",
		},
		{
			name:   "cidr_24",
			length: 24,
			want:   "255.255.255.0",
		},
		{
			name:   "cidr_32",
			length: 32,
			want:   "255.255.255.255",
		},
		{
			name:   "cidr_0",
			length: 0,
			want:   "0.0.0.0",
		},
		{
			name:   "cidr_30",
			length: 30,
			want:   "255.255.255.252",
		},
		{
			name:   "cidr_invalid_negative",
			length: -1,
			want:   "",
		},
		{
			name:   "cidr_invalid_too_large",
			length: 33,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CIDRToDDN(tt.length)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLastHostIPInSubnet(t *testing.T) {
	tests := []struct {
		name   string
		subnet string
		want   string
	}{
		{
			name:   "ipv4_slash_24",
			subnet: "172.20.20.0/24",
			want:   "172.20.20.254",
		},
		{
			name:   "ipv4_slash_16",
			subnet: "10.0.0.0/16",
			want:   "10.0.255.254",
		},
		{
			name:   "ipv4_slash_8",
			subnet: "192.0.0.0/8",
			want:   "192.255.255.254",
		},
		{
			name:   "ipv4_slash_30",
			subnet: "192.168.1.0/30",
			want:   "192.168.1.2",
		},
		{
			name:   "ipv4_slash_31",
			subnet: "192.168.1.0/31",
			want:   "192.168.1.1",
		},
		{
			name:   "ipv4_slash_32",
			subnet: "192.168.1.1/32",
			want:   "192.168.1.1",
		},
		{
			name:   "ipv6_slash_64",
			subnet: "3fff:172:20:20::/64",
			want:   "3fff:172:20:20:ffff:ffff:ffff:ffff",
		},
		{
			name:   "ipv6_slash_48",
			subnet: "fd00:1234:5678::/48",
			want:   "fd00:1234:5678:ffff:ffff:ffff:ffff:ffff",
		},
		{
			name:   "ipv6_slash_128",
			subnet: "2001:db8::1/128",
			want:   "2001:db8::1",
		},
		{
			name:   "ipv4_100_64",
			subnet: "100.64.0.0/24",
			want:   "100.64.0.254",
		},
		{
			name:   "ipv4_slash_25",
			subnet: "172.20.20.0/25",
			want:   "172.20.20.126",
		},
		{
			name:   "ipv4_slash_31_different_base",
			subnet: "10.0.0.254/31",
			want:   "10.0.0.255",
		},
		{
			name:   "ipv4_slash_32_different_ip",
			subnet: "10.10.10.10/32",
			want:   "10.10.10.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipnet, err := net.ParseCIDR(tt.subnet)
			if err != nil {
				t.Fatalf("failed to parse CIDR %q: %v", tt.subnet, err)
			}

			got := LastHostIPInSubnet(ipnet)
			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
