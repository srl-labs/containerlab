// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

// ConvertEnvs converts env variables passed as a map to a list of them.
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

// MergeMaps merges all dictionaries and return a new dictionary
// recursively if matching keys are both dictionaries.
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

// MergeStringMaps merges all string maps and return a new map
// maps that are passed for merging will not be changed
// merging to empty maps return an empty map
// merging nils return nil.
func MergeStringMaps(maps ...map[string]string) map[string]string {
	res := map[string]string{}

	nonNilMapSeen := false // flag to monitor if a non nil map was passed
	for _, m := range maps {
		if m == nil {
			continue
		}

		nonNilMapSeen = true

		for k, v := range m {
			res[k] = v
		}
	}

	// return nil nil instead of an empty map if all maps were nil
	if !nonNilMapSeen {
		return nil
	}

	return res
}

// StringInSlice checks if a slice contains `val` string and returns slice index if true.
func StringInSlice(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

// LoadEnvVarFiles load EnvVars from the given files, resolving relative paths.
func LoadEnvVarFiles(basefolder string, files []string) (map[string]string, error) {
	resolvedPaths := []string{}
	// resolve given paths, relative (to topology definition file)
	for _, file := range files {
		resolved := ResolvePath(file, basefolder)
		if !FileExists(resolved) {
			return nil, fmt.Errorf("env-file %s not found (path resolved to %s)", file, resolved)
		}
		resolvedPaths = append(resolvedPaths, resolved)
	}

	if len(resolvedPaths) == 0 {
		return map[string]string{}, nil
	}

	result, err := godotenv.Read(resolvedPaths...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// MergeStringSlices merges string slices with duplicates removed.
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

// ExpandEnvVarsInStrSlice makes an in-place expansion of env vars in a slice of strings.
func ExpandEnvVarsInStrSlice(s []string) {
	for i, e := range s {
		s[i] = os.ExpandEnv(e)
	}
}

// ToEnvKey capitalizes and removes special chars from a string to is used as an environment variable key.
func ToEnvKey(s string) string {
	// match special chars to later replace with "_"
	regreplace := regexp.MustCompile("[-+./]")
	result := regreplace.ReplaceAllString(s, "_")
	// match only valid env var chars
	regAllowed := regexp.MustCompile("[^a-zA-Z0-9_]+")
	result = regAllowed.ReplaceAllString(result, "")

	return strings.ToUpper(result)
}
