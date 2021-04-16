package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"inet.af/netaddr"
)

func typeof(val interface{}) string {
	switch val.(type) {
	case string:
		return "string"
	case int, int16, int32:
		return "int"
	}
	return ""
}

func hasInt(val interface{}) (int, bool) {
	if i, err := strconv.Atoi(fmt.Sprintf("%v", val)); err == nil {
		return i, true
	}
	return 0, false
}

func expectFunc(val interface{}, format string) (interface{}, error) {
	t := typeof(val)
	vals := fmt.Sprintf("%s", val)

	// known formats
	switch format {
	case "str", "string":
		if t == "string" {
			return "", nil
		}
		return "", fmt.Errorf("string expected, got %s (%v)", t, val)
	case "int":
		if _, ok := hasInt(val); ok {
			return "", nil
		}
		return "", fmt.Errorf("int expected, got %s (%v)", t, val)
	case "ip":
		if _, err := netaddr.ParseIPPrefix(vals); err == nil {
			return "", nil
		}
		return "", fmt.Errorf("IP/mask expected, got %v", val)
	}

	// try range
	if matched, _ := regexp.MatchString(`\d+-\d+`, format); matched {
		iv, ok := hasInt(val)
		if !ok {
			return "", fmt.Errorf("int expected, got %s (%v)", t, val)
		}
		r := strings.Split(format, "-")
		i0, _ := hasInt(r[0])
		i1, _ := hasInt(r[1])
		if i1 < i0 {
			i0, i1 = i1, i0
		}
		if i0 <= iv && iv <= i1 {
			return "", nil
		}
		return "", fmt.Errorf("value (%d) expected to be in range %d-%d", iv, i0, i1)
	}

	// Try regex
	matched, err := regexp.MatchString(format, vals)
	if err != nil || !matched {
		return "", fmt.Errorf("value %s does not match regex %s %v", vals, format, err)
	}

	return "", nil
}

var funcMap = map[string]interface{}{
	"optional": func(val interface{}, format string) (interface{}, error) {
		if val == nil {
			return "", nil
		}
		return expectFunc(val, format)
	},
	"expect": expectFunc,
	// "require": func(val interface{}) (interface{}, error) {
	// 	if val == nil {
	// 		return nil, errors.New("required value not set")
	// 	}
	// 	return val, nil
	// },
	"ip": func(val interface{}) (interface{}, error) {
		s := fmt.Sprintf("%v", val)
		a := strings.Split(s, "/")
		return a[0], nil
	},
	"ipmask": func(val interface{}) (interface{}, error) {
		s := fmt.Sprintf("%v", val)
		a := strings.Split(s, "/")
		return a[1], nil
	},
	"default": func(in ...interface{}) (interface{}, error) {
		if len(in) < 2 {
			return nil, fmt.Errorf("default value expected")
		}
		if len(in) > 2 {
			return nil, fmt.Errorf("too many arguments")
		}

		val := in[len(in)-1]
		def := in[0]

		switch v := val.(type) {
		case nil:
			return def, nil
		case string:
			if v == "" {
				return def, nil
			}
		case bool:
			if !v {
				return def, nil
			}
		}
		// if val == nil {
		// 	return def, nil
		// }

		// If we have a input value, do some type checking
		tval, tdef := typeof(val), typeof(def)
		if tval == "string" && tdef == "int" {
			if _, err := strconv.Atoi(val.(string)); err == nil {
				tval = "int"
			}
			if tdef == "str" {
				if _, err := strconv.Atoi(def.(string)); err == nil {
					tdef = "int"
				}
			}
		}
		if tdef != tval {
			return val, fmt.Errorf("expected type %v, got %v (value=%v)", tdef, tval, val)
		}

		// Return the value
		return val, nil
	},
	"contains": func(substr string, str string) (interface{}, error) {
		return strings.Contains(fmt.Sprintf("%v", str), fmt.Sprintf("%v", substr)), nil
	},
	"split": func(sep string, val interface{}) (interface{}, error) {
		// Start and end values
		if val == nil {
			return []interface{}{}, nil
		}
		if sep == "" {
			sep = " "
		}

		v := fmt.Sprintf("%v", val)

		res := strings.Split(v, sep)
		r := make([]interface{}, len(res))
		for i, p := range res {
			r[i] = p
		}
		return r, nil
	},
	"join": func(sep string, val interface{}) (interface{}, error) {
		if sep == "" {
			sep = " "
		}
		// Start and end values
		switch v := val.(type) {
		case []interface{}:
			if val == nil {
				return "", nil
			}
			res := make([]string, len(v))
			for i, v := range v {
				res[i] = fmt.Sprintf("%v", v)
			}
			return strings.Join(res, sep), nil
		case []string:
			return strings.Join(v, sep), nil
		case []int, []int16, []int32:
			return strings.Trim(strings.ReplaceAll(fmt.Sprint(v), " ", sep), "[]"), nil
		}
		return nil, fmt.Errorf("expected array [], got %v", val)
	},
	"slice": func(start, end int, val interface{}) (interface{}, error) {
		// string or array
		switch v := val.(type) {
		case string:
			if start < 0 {
				start += len(v)
			}
			if end < 0 {
				end += len(v)
			}
			return v[start:end], nil
		case []interface{}:
			if start < 0 {
				start += len(v)
			}
			if end < 0 {
				end += len(v)
			}
			return v[start:end], nil
		}
		return nil, fmt.Errorf("not an array")
	},
	"index": func(idx int, val interface{}) (interface{}, error) {
		// string or array
		switch v := val.(type) {
		case string:
			if idx < 0 {
				idx += len(v)
			}
			return v[idx], nil
		case []interface{}:
			if idx < 0 {
				idx += len(v)
			}
			return v[idx], nil
		}
		return nil, fmt.Errorf("not an array")
	},
}
