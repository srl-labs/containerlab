package template

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"net/netip"
)

// The expect function tests if the input satisfies a certain type.
// If successful it returns nothing and will not affect your template
// If the type fails an error will be raised and template execution stopped
//
// Expect can check the following types:
//
// • a simple type: str, string, int
//
// • and IP address with mask IP/mask
//
// • a numeric range 0-100
//
// • a regular expression
func Expect(val interface{}, format string) (interface{}, error) {
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
		if _, ok := parseInt_i(val); ok {
			return "", nil
		}
		return "", fmt.Errorf("int expected, got %s (%v)", t, val)
	case "ip":

		if _, err := netip.ParsePrefix(vals); err == nil {
			return "", nil
		}
		return "", fmt.Errorf("IP/mask expected, got %v", val)
	}

	// try range
	if matched, _ := regexp.MatchString(`\d+-\d+`, format); matched {
		iv, ok := parseInt_i(val)
		if !ok {
			return "", fmt.Errorf("int expected, got %s (%v)", t, val)
		}
		r := strings.Split(format, "-")
		i0, _ := parseInt_i(r[0])
		i1, _ := parseInt_i(r[1])
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

// The optional function takes exactly the sameparameters as the expect function
// If the value is not supplied, the function will return an empty string
// If a value was supplied, it should match the logic from the expect function
func Optional(val interface{}, format string) (interface{}, error) {
	if val == nil {
		return "", nil
	}
	return Expect(val, format)
}

// interface{} version of parseInt
// parse an integer from val and return the integer if successful, or false if not
func parseInt_i(val interface{}) (int, bool) {
	if i, err := strconv.Atoi(fmt.Sprintf("%v", val)); err == nil {
		return i, true
	}
	return 0, false
}

// Return the type as a string, simlar to the Javascript typeof() function
func typeof(val interface{}) string {
	switch val.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "int"
	}
	return ""
}
