// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// load EnvVars from files
func LoadEnvVarFiles(baseDir string, files []string) (map[string]string, error) {
	result := map[string]string{}
	// iterate over filesnames
	for _, file := range files {
		// if not a root based path, we take it relative from the xyz.clab.yml file
		if file[0] != '/' {
			file = filepath.Join(baseDir, file)
		}

		// read file content
		lines, err := ReadFileLines(file)
		if err != nil {
			return nil, err
		}
		// disect the string slices into maps with environment keys and values
		envMap, err := envVarLineToMap(lines)
		if err != nil {
			return nil, err
		}
		// merge the actual file content with the overall result
		result = MergeStringMaps(result, envMap)
	}
	return result, nil
}

// envVarLineToMap splits the env variable definiton lines into key and value
func envVarLineToMap(lines []string) (map[string]string, error) {
	result := map[string]string{}
	// iterate over lines
	for _, line := range lines {
		// split on the = sign
		splitSlice := strings.Split(line, "=")
		// we expect to see at least two elements in the slice (one = sign present)
		if len(splitSlice) < 2 {
			return nil, fmt.Errorf("issue with format of env file line '%s'", line)
		}
		// take the first element as the key and join the rest back with = as the value
		result[splitSlice[0]] = strings.Join(splitSlice[1:], "=")
	}
	return result, nil
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
