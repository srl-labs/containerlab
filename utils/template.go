package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/hellt/envsubst"
)

var TemplateFuncs = template.FuncMap{
	"ToJSON":       toJson,
	"ToJSONPretty": toJsonPretty,
	"add":          add,
	"subtract":     subtract,
	"seq":          seq,
}

func toJson(v any) string {
	a, _ := json.Marshal(v)

	return string(a)
}

func toJsonPretty(v any, prefix, indent string) string {
	a, _ := json.MarshalIndent(v, prefix, indent)
	return string(a)
}

func add(a, b int) int {
	return a + b
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

	t, err := template.New("template").Funcs(TemplateFuncs).Parse(string(b))
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)

	t.Execute(buf, data)

	return buf, nil
}
