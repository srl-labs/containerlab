package config

import (
	"fmt"
	"testing"
)

func TestFuncMapDefault(t *testing.T) {

	// parameters & return
	tSet := map[string][][]interface{}{
		"default": {
			{0, nil, nil, true},
			{5, 1, 5},
			{5, "1", 5, "invalid types"},
			{"a", 1, "a", "invalid types"},
			{"", "1", "1"},
			{nil, nil, nil, true},
		},
		"contains": {
			{"aa.", ".", true},
			{"ss", ".", false},
		},
		"split": {
			{"a.a", ".", "[a a]"},
			{"a bb", nil, "[a bb]"},
			{nil, nil, "[]"},
			{nil, ".", "[]"},
		},
		"join": {
			{[]interface{}{"a", "b"}, ".", "a.b"},
			{[]string{"a", "b"}, ".", "a.b"},
			{[]int{1, 2}, ".", "1.2"},
		}}

	for name, set := range tSet {
		fn := funcMap[name].(func(interface{}, interface{}) (interface{}, error))

		for _, p := range set {
			// Execute the funciton
			res, err := fn(p[0], p[1])

			// expect return value
			exp := p[2]
			exp_err := len(p) == 4

			// Check errors
			if err != nil && !exp_err {
				t.Errorf("%v no err expected, err: %v", p, err)
			}
			if err == nil && exp_err {
				t.Errorf("%v err expected, non found", p)
			}

			// Check value
			if res != exp {
				// allow arrays (match on string only)
				if fmt.Sprintf("%v", res) == fmt.Sprintf("%v", exp) {
					continue
				}
				t.Errorf("%v expected %v got %v", p, exp, res)
			}
		}
	}
}
