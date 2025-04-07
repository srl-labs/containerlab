package utils

import (
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestStringsSplit(t *testing.T) {
	tests := map[string]struct {
		separator string
		input     any
		want      []string
	}{
		"split string with comma": {
			separator: ",",
			input:     "a,b,c",
			want:      []string{"a", "b", "c"},
		},
		"split string with space": {
			separator: " ",
			input:     "hello world test",
			want:      []string{"hello", "world", "test"},
		},
		"split empty string": {
			separator: ",",
			input:     "",
			want:      []string{""},
		},
		"split with empty separator": {
			separator: "",
			input:     "abc",
			want:      []string{"a", "b", "c"},
		},
		"split non-string input": {
			separator: ",",
			input:     123,
			want:      []string{"123"},
		},
		"split with multi-char separator": {
			separator: "||",
			input:     "a||b||c",
			want:      []string{"a", "b", "c"},
		},
		"split with no separator matches": {
			separator: "|",
			input:     "abc",
			want:      []string{"abc"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sf := &StringFuncs{}
			got := sf.Split(tc.separator, tc.input)
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %v, got %v", tc.want, got)
			}
		})
	}
}
func TestSeq(t *testing.T) {
	tests := map[string]struct {
		args []any
		want []int64
		err  string
	}{
		"zero step returns empty sequence": {
			args: []any{1, 10, 0},
			want: []int64{},
		},
		"negative start and end": {
			args: []any{-5, -1},
			want: []int64{-5, -4, -3, -2, -1},
		},
		"negative step": {
			args: []any{10, 1, -3},
			want: []int64{10, 7, 4, 1},
		},
		"single large number": {
			args: []any{20},
			want: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		},
		"invalid first argument": {
			args: []any{"invalid"},
			err:  "expected a number",
		},
		"invalid second argument": {
			args: []any{1, "invalid"},
			err:  "expected a number",
		},
		"invalid third argument": {
			args: []any{1, 5, "invalid"},
			err:  "expected a number",
		},
		"too many arguments": {
			args: []any{1, 2, 3, 4},
			err:  "expected 1, 2, or 3 arguments, got 4",
		},
		"step correction for ascending": {
			args: []any{1, 10, -2},
			want: []int64{1, 3, 5, 7, 9},
		},
		"step correction for descending": {
			args: []any{10, 1, 2},
			want: []int64{10, 8, 6, 4, 2},
		},
		"float numbers converted to int": {
			args: []any{1.5, 5.7, 2.1},
			want: []int64{1, 3, 5},
		},
		"single element sequence": {
			args: []any{5, 5, 1},
			want: []int64{5},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := seq(tc.args...)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.err)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error containing %q, got %q", tc.err, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %v, got %v", tc.want, got)
			}
		})
	}
}
func TestSubtract(t *testing.T) {
	tests := map[string]struct {
		a    any
		b    any
		want any
		err  string
	}{
		"integer subtraction": {
			a:    10,
			b:    3,
			want: int64(7),
		},
		"float subtraction": {
			a:    10.5,
			b:    3.2,
			want: 7.3,
		},
		"mixed float and integer": {
			a:    10,
			b:    3.5,
			want: 6.5,
		},
		"negative numbers": {
			a:    -5,
			b:    -3,
			want: int64(-2),
		},
		"zero result": {
			a:    5,
			b:    5,
			want: int64(0),
		},
		"first argument invalid": {
			a:   "invalid",
			b:   5,
			err: "expected a number",
		},
		"second argument invalid": {
			a:   5,
			b:   "invalid",
			err: "expected a number",
		},
		"large numbers": {
			a:    9999999999,
			b:    8888888888,
			want: int64(1111111111),
		},
		"decimal precision": {
			a:    3.14159,
			b:    2.0,
			want: 1.1415899999999999,
		},
		"string numbers": {
			a:    "10",
			b:    "3",
			want: int64(7),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := subtract(tc.a, tc.b)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.err)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error containing %q, got %q", tc.err, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %v, got %v", tc.want, got)
			}
		})
	}
}
func TestAdd(t *testing.T) {
	tests := map[string]struct {
		a    int
		b    int
		want int
	}{
		"add positive numbers": {
			a:    5,
			b:    3,
			want: 8,
		},
		"add negative numbers": {
			a:    -2,
			b:    -4,
			want: -6,
		},
		"add positive and negative": {
			a:    10,
			b:    -5,
			want: 5,
		},
		"add zero values": {
			a:    0,
			b:    0,
			want: 0,
		},
		"add max int with small number": {
			a:    2147483647,
			b:    1,
			want: 2147483648,
		},
		"add min int with small number": {
			a:    -2147483648,
			b:    -1,
			want: -2147483649,
		},
		"add same numbers": {
			a:    42,
			b:    42,
			want: 84,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := add(tc.a, tc.b)
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %v, got %v", tc.want, got)
			}
		})
	}
}
func TestToJsonPretty(t *testing.T) {
	tests := map[string]struct {
		input  any
		prefix string
		indent string
		want   string
	}{
		"simple map with default formatting": {
			input:  map[string]string{"key": "value"},
			prefix: "",
			indent: "  ",
			want:   "{\n  \"key\": \"value\"\n}",
		},
		"nested structure with custom prefix": {
			input: map[string]any{
				"outer": map[string]int{"inner": 42},
			},
			prefix: ">>",
			indent: "\t",
			want:   "{\n>>\t\"outer\": {\n>>\t\t\"inner\": 42\n>>\t}\n>>}",
		},
		"array with custom indent": {
			input:  []int{1, 2, 3},
			prefix: "",
			indent: "    ",
			want:   "[\n    1,\n    2,\n    3\n]",
		},
		"empty object": {
			input:  map[string]string{},
			prefix: "",
			indent: "  ",
			want:   "{}",
		},
		"null value": {
			input:  nil,
			prefix: "",
			indent: "  ",
			want:   "null",
		},
		"mixed types": {
			input: map[string]any{
				"string": "text",
				"number": 123,
				"bool":   true,
				"null":   nil,
			},
			prefix: "-",
			indent: " ",
			want:   "{\n- \"bool\": true,\n- \"null\": null,\n- \"number\": 123,\n- \"string\": \"text\"\n-}",
		},
		"special characters": {
			input: map[string]string{
				"escaped\"quote": "new\nline",
			},
			prefix: "",
			indent: " ",
			want:   "{\n \"escaped\\\"quote\": \"new\\nline\"\n}",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := toJsonPretty(tc.input, tc.prefix, tc.indent)
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %q, got %q", tc.want, got)
			}
		})
	}
}
func TestToJson(t *testing.T) {
	tests := map[string]struct {
		input any
		want  string
	}{
		"empty slice": {
			input: []string{},
			want:  "[]",
		},
		"complex nested structure": {
			input: map[string]any{
				"array": []int{1, 2, 3},
				"nested": map[string]any{
					"bool":   false,
					"string": "test",
				},
			},
			want: `{"array":[1,2,3],"nested":{"bool":false,"string":"test"}}`,
		},
		"unicode characters": {
			input: map[string]string{"emoji": "🚀", "unicode": "über"},
			want:  `{"emoji":"🚀","unicode":"über"}`,
		},
		"special json characters": {
			input: map[string]string{"quotes": "\"hello\"", "backslash": "\\path\\"},
			want:  `{"backslash":"\\path\\","quotes":"\"hello\""}`,
		},
		"numeric types": {
			input: map[string]any{
				"integer": 42,
				"float":   3.14,
				"exp":     1.23e-4,
			},
			want: `{"exp":0.000123,"float":3.14,"integer":42}`,
		},
		"boolean values": {
			input: map[string]bool{"true": true, "false": false},
			want:  `{"false":false,"true":true}`,
		},
		"null value": {
			input: map[string]any{"null": nil},
			want:  `{"null":null}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := toJson(tc.input)
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %q, got %q", tc.want, got)
			}
		})
	}
}
func TestStringsReplaceAll(t *testing.T) {
	tests := map[string]struct {
		old  string
		new  string
		s    any
		want string
	}{
		"replace in middle of string": {
			old:  "def",
			new:  "xyz",
			s:    "abcdefghi",
			want: "abcxyzghi",
		},
		"replace multiple occurrences": {
			old:  "a",
			new:  "x",
			s:    "banana",
			want: "bxnxnx",
		},
		"replace with empty string": {
			old:  "test",
			new:  "",
			s:    "testingtesttest",
			want: "ing",
		},
		"replace with longer string": {
			old:  "x",
			new:  "yyy",
			s:    "x-x-x",
			want: "yyy-yyy-yyy",
		},
		"replace in numeric input": {
			old:  "1",
			new:  "one",
			s:    12321,
			want: "one232one",
		},
		"no matches in string": {
			old:  "xyz",
			new:  "abc",
			s:    "test string",
			want: "test string",
		},
		"replace in boolean input": {
			old:  "true",
			new:  "yes",
			s:    true,
			want: "yes",
		},
		"empty old string": {
			old:  "",
			new:  "x",
			s:    "test",
			want: "test",
		},
		"nil input": {
			old:  "test",
			new:  "x",
			s:    nil,
			want: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			sf := &StringFuncs{}
			got := sf.ReplaceAll(tc.old, tc.new, tc.s)
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %q, got %q", tc.want, got)
			}
		})
	}
}
func TestConvFuncsJoin(t *testing.T) {
	tests := map[string]struct {
		input any
		sep   string
		want  string
		err   string
	}{
		"slice of integers": {
			input: []int{1, 2, 3, 4},
			sep:   "-",
			want:  "1-2-3-4",
		},
		"slice of mixed types": {
			input: []any{1, "two", true, 4.5},
			sep:   ", ",
			want:  "1, two, true, 4.5",
		},
		"empty slice": {
			input: []string{},
			sep:   ",",
			want:  "",
		},
		"single element slice": {
			input: []any{"solo"},
			sep:   "---",
			want:  "solo",
		},
		"slice with nil elements": {
			input: []any{nil, "test", nil},
			sep:   "|",
			want:  "nil|test|nil",
		},
		"non-slice input": {
			input: "not a slice",
			sep:   ",",
			err:   "input to Join must be an array",
		},
		"slice with empty strings": {
			input: []string{"", "", ""},
			sep:   ",",
			want:  ",,",
		},
		"complex separator": {
			input: []int{1, 2, 3},
			sep:   "<==>",
			want:  "1<==>2<==>3",
		},
		"slice of booleans": {
			input: []bool{true, false, true},
			sep:   " and ",
			want:  "true and false and true",
		},
		"slice of floats": {
			input: []float64{1.1, 2.2, 3.3},
			sep:   ";",
			want:  "1.1;2.2;3.3",
		},
		"nil input": {
			input: nil,
			sep:   ",",
			err:   "input to Join must be an array",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cf := ConvFuncs{}
			got, err := cf.Join(tc.input, tc.sep)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.err)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error containing %q, got %q", tc.err, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %q, got %q", tc.want, got)
			}
		})
	}
}
func TestConvFuncsToInt(t *testing.T) {
	tests := map[string]struct {
		input any
		want  int
		err   string
	}{
		"max int value": {
			input: math.MaxInt,
			want:  math.MaxInt,
		},
		"min int value": {
			input: math.MinInt,
			want:  math.MinInt,
		},
		"float64 with decimal places": {
			input: 123.456,
			want:  123,
		},
		"hex string": {
			input: "0xFF",
			want:  255,
		},
		"octal string": {
			input: "0o777",
			want:  511,
		},
		"binary string": {
			input: "0b1010",
			want:  10,
		},
		"scientific notation string": {
			input: "1e5",
			want:  100000,
		},
		"invalid hex string": {
			input: "0xZZ",
			err:   "could not convert",
		},
		"empty string": {
			input: "",
			err:   "could not convert",
		},
		"complex number": {
			input: complex(1, 2),
			err:   "could not convert",
		},
		"channel input": {
			input: make(chan int),
			err:   "could not convert",
		},
		"function input": {
			input: func() {},
			err:   "could not convert",
		},
		"map input": {
			input: map[string]int{},
			err:   "could not convert",
		},
		"slice input": {
			input: []int{1, 2, 3},
			err:   "could not convert",
		},
		"too large uint64": {
			input: uint64(math.MaxUint64),
			err:   "could not convert",
		},
		"pointer input": {
			input: &struct{}{},
			err:   "could not convert",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cf := ConvFuncs{}
			got, err := cf.ToInt(tc.input)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.err)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error containing %q, got %q", tc.err, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %v, got %v", tc.want, got)
			}
		})
	}
}
