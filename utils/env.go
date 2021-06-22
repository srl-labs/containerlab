// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

// convertEnvs convert env variables passed as a map to a list of them
func ConvertEnvs(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, k+"="+v)
	}
	return s
}

// mergeStringMaps merges map m1 into m2 and return a resulting map as a new map
// maps that are passed for merging will not be changed
func MergeStringMaps(m1, m2 map[string]string) map[string]string {
	if m1 == nil {
		return m2
	}
	if m2 == nil {
		return m1
	}
	// make a copy of a map
	m := make(map[string]string)
	for k, v := range m1 {
		m[k] = v
	}

	for k, v := range m2 {
		m[k] = v
	}
	return m
}

func StringInSlice(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}
