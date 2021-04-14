package config

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"text/template"
)

var test_set = map[string][]interface{}{
	// empty values
	"default .x 0":    {"0"},
	"default \"\" 0":  {"0"},
	"default false 0": {"0"},
	// ints pass through ok
	"default 0 1": {"0"},
	// errors
	"default .x":     {"", "default value expected"},
	"default .x 1 1": {"", "too many arguments"},
	// type check
	"default .i5 0":     {"5"},
	"default .sA 0":     {"", "expected type int"},
	"default .sA \"5\"": {"A"},

	"contains .sAAA \".\"": {"true"},
	"contains .sA \".\"":   {"false"},

	"split \"a.a\" \".\"":  {"[a a]"},
	"split \"a bb\" \" \"": {"[a bb]"},
	"split \"a bb\" 0":     {"[a bb]"},

	"ip \"1.1.1.1/32\"":     {"1.1.1.1"},
	"ipmask \"1.1.1.1/32\"": {"32"},

	//"split \"a bb\" \" \" | join \"-\"": {"a-bb"},
}

// 		"split": {
// 			{nil, nil, "[]"},
// 			{nil, ".", "[]"},
// 		},
// 		"join": {
// 			{[]interface{}{"a", "b"}, ".", "a.b"},
// 			{[]string{"a", "b"}, ".", "a.b"},
// 			{[]int{1, 2}, ".", "1.2"},
// 		}}

func render(templateS string, vars stringMap) (string, error) {
	var err error
	buf := new(bytes.Buffer)
	ts := fmt.Sprintf("{{ %v }}", strings.Trim(templateS, "{} "))
	tem, err := template.New("").Funcs(funcMap).Parse(ts)
	if err != nil {
		return "invalide template", fmt.Errorf("invalid template")
	}
	err = tem.Execute(buf, vars)
	return buf.String(), err
}

func TestRender1(t *testing.T) {

	l := stringMap{
		"i5":    "5",
		"sA":    "A",
		"sAAA":  "aa.",
		"v_str": "s",
	}

	for tem, exp := range test_set {
		res, err := render(tem, l)

		ss := fmt.Sprintf("{{ %v }} = %v", tem, res)
		if err != nil {
			ss += " error"
		}

		exp_err := len(exp) > 1
		// Check errors
		if exp_err {
			if err == nil {
				t.Errorf("%s: expected '%s' in error, non found", ss, exp[1])
			} else if !strings.Contains(fmt.Sprintf("%v", err), fmt.Sprintf("%v", exp[1])) {
				t.Errorf("%s: expected '%s' in error, got %s", ss, exp[1], err)
			}
		} else if err != nil {
			t.Errorf("%s: no err expected, got %s", ss, err)
		}

		// Check value
		if res != exp[0] {
			t.Errorf("%s: expected %v got %v", ss, exp[0], res)
		}

	}

}
