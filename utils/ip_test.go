package utils

import (
	"net"
	"testing"
)

func TestGetRoutableAddresses(t *testing.T) {
	addrs, err := GetRoutableAddresses()
	if err != nil {
		t.Errorf("GetRoutableAddresses() error = %v", err)
		return
	}

	// Validate that returned addresses are valid IP addresses
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Errorf("GetRoutableAddresses() returned invalid IP address: %s", addr)
		}

		// Verify that returned addresses are not loopback
		if ip.IsLoopback() {
			t.Errorf("GetRoutableAddresses() returned loopback address: %s", addr)
		}

		// Verify that returned addresses are not link-local
		if ip.IsLinkLocalUnicast() {
			t.Errorf("GetRoutableAddresses() returned link-local address: %s", addr)
		}

		// Verify that returned addresses are not multicast
		if ip.IsMulticast() {
			t.Errorf("GetRoutableAddresses() returned multicast address: %s", addr)
		}

		// Verify that returned addresses are not unspecified
		if ip.IsUnspecified() {
			t.Errorf("GetRoutableAddresses() returned unspecified address: %s", addr)
		}
	}

	t.Logf("Found %d routable addresses: %v", len(addrs), addrs)
}