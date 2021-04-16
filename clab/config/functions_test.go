package config

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"text/template"
)

var test_set = map[string][]string{
	// empty values
	"default 0 .x":      {"0"},
	".x | default 0":    {"0"},
	"default 0 \"\"":    {"0"},
	"default 0 false":   {"0"},
	"false | default 0": {"0"},
	// ints pass through ok
	"default 1 0": {"0"},
	// errors
	"default .x":     {"", "default value expected"},
	"default .x 1 1": {"", "too many arguments"},
	// type check
	"default 0 .i5":   {"5"},
	"default 0 .sA":   {"", "expected type int"},
	`default "5" .sA`: {"A"},

	`contains "." .sAAA`:   {"true"},
	`.sAAA | contains "."`: {"true"},
	`contains "." .sA`:     {"false"},
	`.sA | contains "."`:   {"false"},

	`split "." "a.a"`:  {"[a a]"},
	`split " " "a bb"`: {"[a bb]"},

	`ip "1.1.1.1/32"`:     {"1.1.1.1"},
	`"1.1.1.1" | ip`:      {"1.1.1.1"},
	`ipmask "1.1.1.1/32"`: {"32"},
	`"1.1.1.1/32" | split "/" | slice 0 1 | join ""`: {"1.1.1.1"},
	`"1.1.1.1/32" | split "/" | slice 1 2 | join ""`: {"32"},

	`split " " "a bb" | join "-"`: {"a-bb"},
	`split "" ""`:                 {"[]"},
	`split "abc" ""`:              {"[]"},

	`"1.1.1.1/32" | split "/" | index 1`:  {"32"},
	`"1.1.1.1/32" | split "/" | index -1`: {"32"},
	`"1.1.1.1/32" | split "/" | index -2`: {"1.1.1.1"},
	`"1.1.1.1/32" | split "/" | index -3`: {"", "out of range"},
	`"1.1.1.1/32" | split "/" | index 2`:  {"", "out of range"},

	`expect "1.1.1.1/32" "ip"`:   {""},
	`expect "1.1.1.1" "ip"`:      {"", "IP/mask"},
	`expect "1" "0-10"`:          {""},
	`expect "1" "10-10"`:         {"", "range"},
	`expect "1.1" "\\d+\\.\\d+"`: {""},
	`expect 11 "\\d"`:            {""},
	`expect 11 "\\d+"`:           {""},
	`expect "abc" "^[a-z]+$"`:    {""},

	`expect 1 "int"`:    {""},
	`expect 1 "str"`:    {"", "string expected"},
	`expect 1 "string"`: {"", "string expected"},
	`expect .i5 "int"`:  {""},
	`expect "5" "int"`:  {""}, // hasInt
	`expect "aa" "int"`: {"", "int expected"},

	`optional 1 "int"`:   {""},
	`optional .x "int"`:  {""},
	`optional .x "str"`:  {""},
	`optional .i5 "str"`: {""}, // corner case, although it hasInt everything is always a string
}

func render(templateS string, vars map[string]string) (string, error) {
	var err error
	buf := new(bytes.Buffer)
	ts := fmt.Sprintf("{{ %v }}", strings.Trim(templateS, "{} "))
	tem, err := template.New("").Funcs(funcMap).Parse(ts)
	if err != nil {
		return "", fmt.Errorf("invalid template")
	}
	err = tem.Execute(buf, vars)
	return buf.String(), err
}

func TestRender1(t *testing.T) {

	l := map[string]string{
		"i5":    "5",
		"sA":    "A",
		"sAAA":  "aa.",
		"dot":   ".",
		"space": " ",
	}

	for tem, exp := range test_set {
		res, err := render(tem, l)

		e := []string{fmt.Sprintf(`{{ %v }} = "%v", error=%v`, tem, res, err)}

		// Check value
		if res != exp[0] {
			e = append(e, fmt.Sprintf("- expected value = %v", exp[0]))
		}

		// Check errors
		if len(exp) > 1 {
			ee := fmt.Sprintf("- expected error with %s", exp[1])
			if err == nil {
				e = append(e, ee)
			} else if !strings.Contains(err.Error(), exp[1]) {
				e = append(e, ee)
			}
		} else if err != nil {
			e = append(e, "- no error expected")
		}

		if len(e) > 1 {
			t.Error(strings.Join(e, "\n"))
		}
	}

}
