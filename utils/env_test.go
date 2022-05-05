package utils

import (
	"runtime"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

type TestConfig struct {
	FirstField  string                 `json:"first,omitempty"`
	SecondField *bool                  `json:"second,omitempty"`
	ThirdField  *int                   `json:"last,omitempty"`
	MapField    map[string]interface{} `json:"map,omitempty"`
	ArrayField  []interface{}          `json:"array,omitempty"`
}

func assert(t *testing.T, val, exp interface{}) {
	if !cmp.Equal(val, exp) {
		_, fn, line, _ := runtime.Caller(1)
		t.Errorf("assert failed on line %v in %s\n%s", line, fn, cmp.Diff(exp, val))
	}
}

func TestMergeMaps(t *testing.T) {
	MergeMaps(nil, nil)
	d1 := map[string]interface{}{
		"t": "1",
	}
	d2 := map[string]interface{}{
		"t":  "2",
		"t2": "1",
	}
	assert(t, MergeMaps(nil, d1), d1)
	assert(t, MergeMaps(d1, d1), d1)
	assert(t, MergeMaps(d1, nil), d1)
	assert(t, MergeMaps(d1, d2), d2)
	assert(t, MergeMaps(d2, d1), map[string]interface{}{
		"t":  "1",
		"t2": "1",
	})
}

func TestMergeMapsRecursive(t *testing.T) {
	d0 := map[string]interface{}{
		"a": "1",
	}
	d1 := map[string]interface{}{
		"a": "11",
		"b": "2",
	}
	r0 := map[string]interface{}{
		"r":  d0,
		"r1": "1",
	}
	r1 := map[string]interface{}{
		"r":  d1,
		"r2": "2",
	}
	r3 := map[string]interface{}{
		"r":  "00",
		"r2": "0",
	}

	exp0 := map[string]interface{}{
		"a": "1",
		"b": "2",
	}
	exp1 := d1

	// all simple vars... second overwrites
	assert(t, MergeMaps(d1, d0), exp0)
	assert(t, MergeMaps(d0, d1), exp1)

	// r are both dicts... recursive on r... same inner result as the previous
	assert(t, MergeMaps(r1, r0), map[string]interface{}{"r": exp0, "r1": "1", "r2": "2"})
	assert(t, MergeMaps(r0, r1), map[string]interface{}{"r": exp1, "r1": "1", "r2": "2"})

	// one is NOT a dict... second overwrites
	assert(t, MergeMaps(r1, r3), map[string]interface{}{"r": "00", "r2": "0"})
	assert(t, MergeMaps(r3, r1), map[string]interface{}{"r": exp1, "r2": "2"})
}

func TestMergeStringMaps(t *testing.T) {
	d0 := map[string]string{
		"a": "1",
	}
	d1 := map[string]string{
		"a": "11",
		"b": "2",
	}

	// all simple vars... second overwrites
	assert(t, MergeStringMaps(d1, d0), map[string]string{"a": "1", "b": "2"})
	assert(t, MergeStringMaps(d0, d1), map[string]string{"a": "11", "b": "2"})
}

func TestMapify(t *testing.T) {
	a := map[interface{}]interface{}{
		"key": "val",
	}
	b, ismap := mapify(a)
	assert(t, ismap, true)
	t.Logf("%v", b)
	assert(t, b, map[string]interface{}{"key": "val"})
}

func TestMergeStructConfigs(t *testing.T) {
	// default config
	d := &TestConfig{
		FirstField: "set in default",
		ThirdField: pointer.ToInt(1),
		MapField:   map[string]interface{}{"a": 123, "c": map[string]interface{}{"z": 321}},
		ArrayField: []interface{}{1, 2},
	}
	// kind sets the 2nd
	k := &TestConfig{
		SecondField: pointer.ToBool(true),
		MapField:    map[string]interface{}{"b": 456, "c": map[string]interface{}{"y": 654}}}
	// node overwrites 1st and 3rd fields
	n := &TestConfig{
		FirstField: "set in node",
		ThirdField: pointer.ToInt(3),
		ArrayField: []interface{}{1, 3}}

	r := MergeStructConfigs(d, k, n).(*TestConfig)

	// json unmarshalling apparently converts literal integers to floats
	// reference: https://github.com/square/go-jose/issues/351
	exp := &TestConfig{
		FirstField:  "set in node",
		SecondField: pointer.ToBool(true),
		ThirdField:  pointer.ToInt(3),
		MapField:    map[string]interface{}{"a": int(123), "b": float64(456), "c": map[string]interface{}{"y": float64(654)}},
		ArrayField:  []interface{}{float64(1), float64(3)},
	}
	assert(t, r, exp)
}

func TestMergeMapsFromYaml(t *testing.T) {
	a := make(map[string]interface{})
	b := make(map[string]interface{})

	a_in := `
globvar: globval
globmap:
  var1: val1
  var2: val2
`
	b_in := `
globmap:
  var2: rewritten
  newvar: newval
interfaces:
  - name: ethernet-1/1
    description: set in node
  - name: ethernet-1/2
`

	err := yaml.Unmarshal([]byte(a_in), a)
	assert(t, err, nil)
	err = yaml.Unmarshal([]byte(b_in), b)
	assert(t, err, nil)

	result := MergeMaps(a, b)
	// We will test this result against:
	//   1. a golang struct (shows exact types)
	//   2. the expected result loaded from yaml

	// 1. expected value in Go
	expG := map[string]interface{}{
		"globvar": "globval",
		"globmap": map[string]interface{}{
			"var1":   "val1",
			"var2":   "rewritten",
			"newvar": "newval",
		},
		"interfaces": []interface{}{
			map[interface{}]interface{}{
				"name":        "ethernet-1/1",
				"description": "set in node",
			},
			map[interface{}]interface{}{
				"name": "ethernet-1/2",
			},
		},
	}
	assert(t, result, expG)

	// 2. expected value as text
	expT_in := `
globvar: globval
globmap:
  var1: val1
  var2: rewritten
  newvar: newval
interfaces:
  - name: ethernet-1/1
    description: set in node
  - name: ethernet-1/2
`

	expT := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(expT_in), expT)
	assert(t, err, nil)

	// Run expT through MergeMaps to convert "map[interface{}]" --> "map[string]"
	// This is only done for maps & maps in maps, NOT for maps in arrays (refer to 1 above)
	expT = MergeMaps(expT)

	assert(t, result, expT)

}

func TestMergeMapsLists(t *testing.T) {
	d1 := map[string]interface{}{
		"t": []string{"1"},
	}
	d2 := map[string]interface{}{
		"t": []string{"2"},
	}
	assert(t, MergeMaps(nil, d1), d1)
	assert(t, MergeMaps(d1, d1), d1)
	assert(t, MergeMaps(d1, nil), d1)
	assert(t, MergeMaps(d1, d2), d2)
}

func TestMergeStringSlices(t *testing.T) {
	type args struct {
		slices [][]string
	}
	tt := map[string]struct {
		got  args
		want []string
	}{
		"three-non-empty-unique-slices": {
			got: args{
				slices: [][]string{{"1", "2"}, {"3"}, {"4", "5"}},
			},
			want: []string{"1", "2", "3", "4", "5"},
		},
		"three-non-empty-non-unique-slices": {
			got: args{
				slices: [][]string{{"1", "2"}, {"1", "3"}, {"2", "4", "5"}},
			},
			want: []string{"1", "2", "3", "4", "5"},
		},
		"three-non-unique-slices-one-empty": {
			got: args{
				slices: [][]string{{"1", "2"}, {}, {"2", "4", "5"}},
			},
			want: []string{"1", "2", "4", "5"},
		},
		"empty-slices": {
			got: args{
				slices: [][]string{{}, {}, nil},
			},
			want: []string{},
		},
		"nil-slices": {
			got: args{
				slices: [][]string{nil, nil},
			},
			want: nil,
		},
	}

	for _, tc := range tt {
		res := MergeStringSlices(tc.got.slices...)
		if !cmp.Equal(res, tc.want) {
			t.Fatalf("wanted %q got %q", tc.want, res)
		}

	}
}
