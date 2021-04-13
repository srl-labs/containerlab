package config

import (
	"fmt"
	"testing"
)

func TestFuncMapDefault(t *testing.T) {

	// parameters & return
	tSet := map[string][][]interface{}{
		"default": {
			{nil, 1, 1},
			{5, 1, 5},
			{"", "1", "1"},
			{nil, nil, fmt.Errorf("")},
		},
		"contains": {
			{"aa.", ".", true},
			{"ss", ".", false},
		}}

	for name, set := range tSet {
		fn := funcMap[name].(func(interface{}, interface{}) (interface{}, error))

		for _, p := range set {
			exp := p[len(p)-1]
			var expe error
			switch v := exp.(type) {
			case error:
				expe = v
				exp = nil
			}
			res, err := fn(p[0], p[1])
			if res != exp || (err != nil && expe == nil) {
				t.Errorf("%v expected %v got %v error %v err: %v", p, exp, res, expe, err)
			}
		}
	}
}
