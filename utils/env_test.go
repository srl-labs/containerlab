package utils

import (
	"fmt"
	"testing"
)

func TestMergeDicts(t *testing.T) {
	MergeDicts(nil, nil)
	d1 := map[string]interface{}{
		"t": "1",
	}
	d2 := map[string]interface{}{
		"t":  "2",
		"t2": "1",
	}
	expect := func(m1, m2 map[string]interface{}, v ...string) {
		d := MergeDicts(m1, m2)
		for i := 0; i < len(v)/2; i++ {
			if fmt.Sprintf("%v", d[v[i*2]]) != v[i*2+1] {
				t.Errorf("err %v, expected %s", d, v)
			}
		}
	}
	expect(nil, d1, "t", "1")
	expect(d1, d1, "t", "1")
	expect(d1, nil, "t", "1")
	expect(d2, d1, "t", "1", "t2", "1")
	expect(d1, d2, "t", "2", "t2", "1")
	expect(nil, d2, "t", "2", "t2", "1")
}

func TestMergeDictsRecursive(t *testing.T) {
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

	expect := func(m1, m2 map[string]interface{}, v string) {
		d := MergeDicts(m1, m2)
		if fmt.Sprintf("%v", d) != v {
			t.Errorf("err %v, expected %s", d, v)
		}
	}
	// all simple vars... second overwrites
	expect(d1, d0, "map[a:1 b:2]")
	expect(d0, d1, "map[a:11 b:2]")

	// r are both dicts... recursive on r... same inner result as the previous
	expect(r1, r0, "map[r:map[a:1 b:2] r1:1 r2:2]")
	expect(r0, r1, "map[r:map[a:11 b:2] r1:1 r2:2]")

	// one is NOT a dict... second overwrites
	expect(r1, r3, "map[r:00 r2:0]")
	expect(r3, r1, "map[r:map[a:11 b:2] r2:2]")
}

func TestMergeStringMaps(t *testing.T) {
	d0 := map[string]string{
		"a": "1",
	}
	d1 := map[string]string{
		"a": "11",
		"b": "2",
	}

	expect := func(m1, m2 map[string]string, v string) {
		d := MergeStringMaps(m1, m2)
		if fmt.Sprintf("%v", d) != v {
			t.Errorf("err %v, expected %s", d, v)
		}
	}
	// all simple vars... second overwrites
	expect(d1, d0, "map[a:1 b:2]")
	expect(d0, d1, "map[a:11 b:2]")
}
