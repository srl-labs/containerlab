package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/hellt/envsubst"
)

func CreateFuncs() template.FuncMap {
	f := template.FuncMap{
		"ToJSON":       toJson,
		"ToJSONPretty": toJsonPretty,
		"add":          add,
		"subtract":     subtract,
		"mul":          mul,
		"div":          div,
		"rem":          rem,
		"seq":          seq,
	}
	maps.Copy(f, CreateStringFuncs())
	maps.Copy(f, CreateConvFuncs())

	return f
}

func toJson(v any) string {
	a, _ := json.Marshal(v)

	return string(a)
}

func toJsonPretty(v any, prefix, indent string) string {
	a, _ := json.MarshalIndent(v, prefix, indent)
	return string(a)
}

// add a to b
// copy from gomplate.
func add(a, b any) (any, error) {
	if containsFloat(a, b) {
		fa, err := ToFloat64(a)
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		fb, err := ToFloat64(b)
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		return fa + fb, nil
	}

	ia, err := ToInt64(a)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	ib, err := ToInt64(b)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	return ia + ib, nil
}

// multiply a by b
// copy from gomplate.
func mul(a, b any) (any, error) {
	if containsFloat(a, b) {
		fa, err := ToFloat64(a)
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		fb, err := ToFloat64(b)
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		return fa * fb, nil
	}

	ia, err := ToInt64(a)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	ib, err := ToInt64(b)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	return ia * ib, nil
}

// subtract b from a
// copy from gomplate.
func subtract(a, b any) (any, error) {
	if containsFloat(a, b) {
		fa, err := ToFloat64(a)
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		fb, err := ToFloat64(b)
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		return fa - fb, nil
	}

	ia, err := ToInt64(a)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	ib, err := ToInt64(b)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	return ia - ib, nil
}

// divide a by b
// copy from gomplate.
func div(a, b any) (any, error) {
	divisor, err := ToFloat64(a)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	dividend, err := ToFloat64(b)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	if dividend == 0 {
		return 0, fmt.Errorf("error: division by 0")
	}

	return divisor / dividend, nil
}

// the remainder of a divided by b
// copy from gomplate.
func rem(a, b any) (any, error) {
	ia, err := ToInt64(a)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	ib, err := ToInt64(b)
	if err != nil {
		return nil, fmt.Errorf("expected a number: %w", err)
	}

	if ib == 0 {
		return nil, fmt.Errorf("expected a number: divisor is zero")
	}

	return ia % ib, nil
}

// Generate number sequence
// Default values: start 1, step 1
// 1 argument: end (end=8 "1 2 3 4 5 6 7 8")
// 2 arguments: start, end (start=4 end=8 "4 5 6 7 8")
// 3 arguments: start, end, step (start=4 end=8 step=2 "4 6 8")
// Also works with counting down (start=8 end=4 step=2 "8 6 4")
// Copied from gomplate.
func seq(n ...any) (any, error) { // skipcq: GO-R1005
	start := int64(1)
	end := int64(0)
	step := int64(1)

	var err error

	switch len(n) {
	case 1:
		end, err = ToInt64(n[0])
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}
	case 2:
		start, err = ToInt64(n[0])
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		end, err = ToInt64(n[1])
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}
	case 3:
		start, err = ToInt64(n[0])
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		end, err = ToInt64(n[1])
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}

		step, err = ToInt64(n[2])
		if err != nil {
			return nil, fmt.Errorf("expected a number: %w", err)
		}
	default:
		return nil, fmt.Errorf("expected 1, 2, or 3 arguments, got %d", len(n))
	}

	// if step is 0, return empty sequence
	if step == 0 {
		return []int64{}, nil
	}

	// handle cases where step has wrong sign
	if end < start && step > 0 {
		step = -step
	}
	if end > start && step < 0 {
		step = -step
	}

	// adjust the end so it aligns exactly (avoids infinite loop!)
	end -= (end - start) % step

	seq := []int64{start}
	last := start
	for last != end {
		last = seq[len(seq)-1] + step
		seq = append(seq, last)
	}
	return seq, nil
}

func containsFloat(n ...any) bool {
	c := false
	for _, v := range n {
		if isFloat(v) {
			return true
		}
	}
	return c
}

func isFloat(n any) bool {
	switch i := n.(type) {
	case float32, float64:
		return true
	case string:
		_, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return false
		}
		if isInt(i) {
			return false
		}
		return true
	}
	return false
}

func isInt(n any) bool {
	switch i := n.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case string:
		_, err := strconv.ParseInt(i, 0, 64)
		return err == nil
	}
	return false
}

func ToFloat64(v interface{}) (float64, error) {
	if str, ok := v.(string); ok {
		return strToFloat64(str)
	}

	val := reflect.Indirect(reflect.ValueOf(v))
	switch val.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return float64(val.Int()), nil
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return float64(val.Uint()), nil
	case reflect.Uint, reflect.Uint64:
		return float64(val.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return val.Float(), nil
	case reflect.Bool:
		if val.Bool() {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("could not convert %v to float64", v)
	}
}

func strToInt64(str string) (int64, error) {
	if strings.Contains(str, ",") {
		str = strings.ReplaceAll(str, ",", "")
	}

	iv, err := strconv.ParseInt(str, 0, 64)
	if err != nil {
		// maybe it's a float?
		var fv float64
		fv, err = strconv.ParseFloat(str, 64)
		if err != nil {
			return 0, fmt.Errorf("could not convert %q to int64: %w", str, err)
		}

		return ToInt64(fv)
	}

	return iv, nil
}

func ToInt64(v interface{}) (int64, error) {
	if str, ok := v.(string); ok {
		return strToInt64(str)
	}

	val := reflect.Indirect(reflect.ValueOf(v))
	switch val.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return val.Int(), nil
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		//nolint:gosec // G115 isn't applicable, this is a Uint32 at most
		return int64(val.Uint()), nil
	case reflect.Uint, reflect.Uint64:
		tv := val.Uint()

		if tv > math.MaxInt64 {
			return 0, fmt.Errorf("could not convert %d to int64, would overflow", tv)
		}

		return int64(tv), nil
	case reflect.Float32, reflect.Float64:
		return int64(val.Float()), nil
	case reflect.Bool:
		if val.Bool() {
			return 1, nil
		}

		return 0, nil
	default:
		return 0, fmt.Errorf("could not convert %v to int64", v)
	}
}

func strToFloat64(str string) (float64, error) {
	if strings.Contains(str, ",") {
		str = strings.ReplaceAll(str, ",", "")
	}

	// this is inefficient, but it's the only way I can think of to
	// properly convert octal integers to floats
	iv, err := strconv.ParseInt(str, 0, 64)
	if err != nil {
		// ok maybe it's a float?
		var fv float64
		fv, err = strconv.ParseFloat(str, 64)
		if err != nil {
			return 0, fmt.Errorf("could not convert %q to float64: %w", str, err)
		}

		return fv, nil
	}

	return float64(iv), nil
}

var (
	errorType       = reflect.TypeOf((*error)(nil)).Elem()
	fmtStringerType = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
)

// indirect returns the item at the end of indirection, and a bool to indicate if it's nil.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
	}
	return v, false
}

// printableValue returns the, possibly indirected, interface value inside v that
// is best for a call to formatted printer.
func printableValue(v reflect.Value) (any, bool) {
	if v.Kind() == reflect.Ptr {
		v, _ = indirect(v) // fmt.Fprint handles nil.
	}
	if !v.IsValid() {
		return "<no value>", true
	}

	if !v.Type().Implements(errorType) && !v.Type().Implements(fmtStringerType) {
		if v.CanAddr() && (reflect.PointerTo(v.Type()).Implements(errorType) ||
			reflect.PointerTo(v.Type()).Implements(fmtStringerType)) {
			v = v.Addr()
		} else {
			switch v.Kind() {
			case reflect.Chan, reflect.Func:
				return nil, false
			}
		}
	}
	return v.Interface(), true
}

func ToString(in any) string {
	if in == nil {
		return "nil"
	}
	if s, ok := in.(string); ok {
		return s
	}
	if s, ok := in.(fmt.Stringer); ok {
		return s.String()
	}
	if s, ok := in.([]byte); ok {
		return string(s)
	}

	v, ok := printableValue(reflect.ValueOf(in))
	if ok {
		in = v
	}

	return fmt.Sprint(in)
}

// CreateStringFuncs returns a new mapping of template StringFuncs.
func CreateStringFuncs() map[string]any {
	f := map[string]any{}

	ns := &StringFuncs{}
	f["strings"] = func() any { return ns }

	return f
}

// StringFuncs holds string related functions for templates.
type StringFuncs struct{}

// Split slices input into the substrings separated by separator, returning a slice of the substrings between those separators. If input does not contain separator and separator is not empty, returns a single-element slice whose only element is input.
// If separator is empty, it will split after each UTF-8 sequence. If both inputs are empty (i.e. strings.Split "" ""), it will return an empty slice.
// This is equivalent to strings.SplitN with a count of -1.
// Note that the delimiter is not included in the resulting elements.
func (sf *StringFuncs) Split(sep string, s any) []string {
	return strings.Split(ToString(s), sep)
}

// ReplaceAll replaces all occurrences of a given string with another.
func (sf *StringFuncs) ReplaceAll(old, replacement string, s any) string {
	if old == "" {
		return ToString(s)
	}
	if s == nil {
		return ""
	}
	return strings.ReplaceAll(ToString(s), old, replacement)
}

// CreateConvFuncs returns a new mapping of template ConvFuncs.
func CreateConvFuncs() map[string]any {
	f := map[string]any{}

	ns := &ConvFuncs{}
	f["conv"] = func() any { return ns }

	return f
}

// ConvFuncs holds conversion related functions for templates.
type ConvFuncs struct{}

// Join concatenates the elements of a to create a single string.
// The separator string sep is placed between elements in the resulting string.
// This is functionally identical to strings.Join, except that each element is
// coerced to a string first.
func (ConvFuncs) Join(in any, sep string) (out string, err error) {
	s, ok := in.([]string)
	if ok {
		return strings.Join(s, sep), nil
	}

	var a []any
	a, ok = in.([]any)
	if !ok {
		a, err = InterfaceSlice(in)
		if err != nil {
			return "", fmt.Errorf("input to Join must be an array: %w", err)
		}
		ok = true
	}
	if ok {
		b := make([]string, len(a))
		for i := range a {
			b[i] = ToString(a[i])
		}
		return strings.Join(b, sep), nil
	}

	return "", fmt.Errorf("input to Join must be an array")
}

// InterfaceSlice converts an array or slice of any type into an []any
// for use in functions that expect this.
func InterfaceSlice(slice any) ([]any, error) {
	// avoid all this nonsense if this is already a []any...
	if s, ok := slice.([]any); ok {
		return s, nil
	}
	s := reflect.ValueOf(slice)
	kind := s.Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		l := s.Len()
		ret := make([]any, l)
		for i := range l {
			ret[i] = s.Index(i).Interface()
		}
		return ret, nil
	default:
		return nil, fmt.Errorf("expected an array or slice, but got a %T", s)
	}
}

// ToInt converts the input to an int (signed integer, 32- or 64-bit depending on platform). This is similar to conv.ToInt64 on 64-bit platforms, but is useful when input to another function must be provided as an int.
// Unconvertible inputs will result in errors.
// On 32-bit systems, given a number that is too large to fit in an int, the result is -1. This is done to protect against CWE-190 and CWE-681.
func (ConvFuncs) ToInt(in any) (int, error) {
	i, err := ToInt64(in)
	if err != nil {
		return 0, err
	}

	// Bounds-checking to protect against CWE-190 and CWE-681
	// https://cwe.mitre.org/data/definitions/190.html
	// https://cwe.mitre.org/data/definitions/681.html
	if i >= math.MinInt && i <= math.MaxInt {
		return int(i), nil
	}

	// maybe we're on a 32-bit system, so we can't represent this number
	return 0, fmt.Errorf("could not convert %v to int", in)
}

// SubstituteEnvsAndTemplate substitutes environment variables and template the reader `r`
// with the `data` template data.
func SubstituteEnvsAndTemplate(r io.Reader, data any) (*bytes.Buffer, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// expand env vars in `b` if any were set
	// do not replace vars initialized with defaults
	// and do not replace vars that are not set
	b, err = envsubst.BytesRestrictedNoReplace(b, false, false, true, true)
	if err != nil {
		return nil, err
	}

	t, err := template.New("template").Funcs(CreateFuncs()).Parse(string(b))
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)

	t.Execute(buf, data)

	return buf, nil
}
