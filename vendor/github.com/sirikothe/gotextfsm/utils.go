package gotextfsm

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

func TrimRightSpace(str string) string {
	return strings.TrimRightFunc(str, func(r rune) bool { return unicode.IsSpace(r) })
}
func GetNamedMatches(r *regexp.Regexp, s string) map[string]string {
	match := r.FindStringSubmatch(s)
	if match == nil {
		return nil
	}
	subMatchMap := make(map[string]string)
	for i, name := range r.SubexpNames() {
		if i != 0 {
			subMatchMap[name] = match[i]
		}
	}
	return subMatchMap
}

// Given a regular expression with named groups
// ex. (?P<name>\w+)\s+(?P<age>\d+)
// Return the names e.g. ["name", "age"]
func GetGroupNames(r string) ([]string, error) {
	r1 := regexp.MustCompile("\\(\\?P\\<([a-z]+)\\>")
	m := r1.FindAllStringSubmatch(r, -1)
	output := make([]string, 0)
	if m == nil || len(m) == 0 {
		return output, nil
	}
	for _, arr := range m {
		if FindIndex(output, arr[1]) >= 0 {
			return nil, fmt.Errorf("Duplicate name '%s'", arr[1])
		}
		output = append(output, arr[1])
	}
	return output, nil
}
func FindIndex(arr []string, elem string) int {
	for i, v := range arr {
		if v == elem {
			return i
		}
	}
	return -1
}
