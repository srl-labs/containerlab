// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
)

// convertEnvs convert env variables passed as a map to a list of them
func ConvertEnvs(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, k+"="+v)
	}
	return s
}

func mapify(i interface{}) (map[string]interface{}, bool) {
	value := reflect.ValueOf(i)
	if value.Kind() == reflect.Map {
		m := map[string]interface{}{}
		for _, k := range value.MapKeys() {
			m[fmt.Sprintf("%v", k)] = value.MapIndex(k).Interface()
		}
		return m, true
	}
	return map[string]interface{}{}, false
}

// merge all dictionaries and return a new dictionary
// recursively if matching keys are both dictionaries
func MergeMaps(dicts ...map[string]interface{}) map[string]interface{} {
	res := make(map[string]interface{})
	for _, m := range dicts {
		if m == nil {
			continue
		}
		for k, v := range m {
			vMap, vMapOk := mapify(v)
			if v0, ok := res[k]; ok {
				// Recursive merging if res[k] exists (and both are dicts)
				t0, ok0 := mapify(v0)
				if ok0 && vMapOk {
					res[k] = MergeMaps(t0, vMap)
					continue
				}
			}
			if vMapOk {
				res[k] = vMap
			} else {
				res[k] = v
			}
		}
	}
	return res
}

// merge all string maps and return a new map
// maps that are passed for merging will not be changed
func MergeStringMaps(maps ...map[string]string) map[string]string {
	res := make(map[string]string)
	for _, m := range maps {
		if m == nil {
			continue
		}
		for k, v := range m {
			res[k] = v
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

// does a slice contain a string
func StringInSlice(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

// MergeStringSlices merges string slices with duplicates removed
func MergeStringSlices(ss ...[]string) []string {
	res := make([]string, 0)
	allNils := true // switch to track if all of the passed slices are nils
	for _, s := range ss {
		res = append(res, s...)
		if s != nil {
			allNils = false
		}
	}

	// if all slices are nil, return nil instead of an empty slice
	if allNils {
		return nil
	}

	m := map[string]struct{}{}
	uniques := make([]string, 0)
	for _, val := range res {
		if _, ok := m[val]; !ok {
			m[val] = struct{}{}
			uniques = append(uniques, val)
		}
	}

	return uniques
}

// ExpandEnvVarsInStrSlice makes an in-place expansion of env vars in a slice of strings
func ExpandEnvVarsInStrSlice(s []string) {
	for i, e := range s {
		s[i] = os.ExpandEnv(e)
	}
}

// ConvertToEnvKey remove specual chars etc. from a string, to be used as environment variable key
func ConvertToEnvKey(s string) string {
	// match spechial chars to later replace with "_"
	regreplace, _ := regexp.Compile("[+-./]")
	result := regreplace.ReplaceAllString(s, "_")
	// match only valid env var chars
	regAllowed, _ := regexp.Compile("[^a-zA-Z0-9_]+")
	result = regAllowed.ReplaceAllString(result, "")

	return result
}
