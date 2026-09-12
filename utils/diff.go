package utils

import (
	"github.com/docker/go-connections/nat"
)

// PortSetsEqual compares two nat.PortSet values, treating nil and empty as equal.
func PortSetsEqual(a, b nat.PortSet) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// SlicesEqualOrBothEmpty treats nil and empty slices as equal.
func SlicesEqualOrBothEmpty[T comparable](a, b []T) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// MapsEqualOrBothEmpty treats nil and empty maps as equal.
func MapsEqualOrBothEmpty[K, V comparable](a, b map[K]V) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
