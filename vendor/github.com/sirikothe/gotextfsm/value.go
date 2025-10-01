package gotextfsm

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

const MAX_NAME_LENG = 48

type ON_RECORD_TYPE int

const (
	SKIP_RECORD ON_RECORD_TYPE = iota
	SKIP_VALUE
	CONTINUE
)

type TextFSMValue struct {
	Regex          string
	Template       string
	Name           string
	Options        []string
	curval         interface{}
	filldown_value interface{}
}

func isValidOption(str string) bool {
	switch str {
	case "Required", "Key", "List", "Filldown", "Fillup":
		return true
	}
	return false
}

func (value *TextFSMValue) Parse(input string, line_num int) error {
	tokens := strings.Fields(input)
	if len(tokens) < 3 {
		return fmt.Errorf("%d Line: Expect at least 3 tokens on line.", line_num)
	}
	value.Options = make([]string, 0)
	if !strings.HasPrefix(tokens[2], "(") {
		// Format: Value Options Name Regular Expression
		// ex: Value Filledown,Required interface (.*)
		options := tokens[1]
		for _, option := range strings.Split(options, ",") {
			if !isValidOption(option) {
				return fmt.Errorf("Line %d: Invalid option %s", line_num, option)
			}
			idx := FindIndex(value.Options, option)
			if idx >= 0 {
				return fmt.Errorf("%d Line: Duplicate option %s", line_num, option)
			}
			value.Options = append(value.Options, option)
		}
		value.Name = tokens[2]
		value.Regex = strings.Join(tokens[3:], " ")
	} else {
		// Format: Value Name Regular Expression
		// ex: Value interface (.*)
		value.Name = tokens[1]
		value.Regex = strings.Join(tokens[2:], " ")
	}
	if len(value.Name) > MAX_NAME_LENG {
		return fmt.Errorf("%d Line: Invalid Value name '%s' or name too long.", line_num, value.Name)
	}
	square_brackets := regexp.MustCompile(`([^\\]?)\[[^]]*]`)
	regex_without_brackets := square_brackets.ReplaceAllString(value.Regex, "$1")
	if !regexp.MustCompile(`^\(.*\)$`).MatchString(value.Regex) {
		return fmt.Errorf("%d Line: Value '%s' must be contained within a '()' pair.", line_num, value.Regex)
	}
	if strings.Count(regex_without_brackets, "(") != strings.Count(regex_without_brackets, ")") {
		return fmt.Errorf("%d Line: Value '%s' must be contained within a '()' pair.", line_num, value.Regex)
	}
	if _, err := regexp.Compile(value.Regex); err != nil {
		return fmt.Errorf("Line %d: Invalid regular expression '%s'. Error: '%s'", line_num, value.Regex, err.Error())
	}
	if _, err := GetGroupNames(value.Regex); err != nil {
		return fmt.Errorf("Line %d: Invalid group names. Error: %s", line_num, err.Error())
	}
	value.Template = regexp.MustCompile(`^\(`).ReplaceAllString(value.Regex, fmt.Sprintf("(?P<%s>", value.Name))
	return nil
}

// String() returns a string representation of the value
func (v *TextFSMValue) String() string {
	var sb strings.Builder
	sb.WriteString("Value ")
	if v.Options != nil && len(v.Options) > 0 {
		sb.WriteString(strings.Join(v.Options, ","))
		sb.WriteString(" ")
	}
	sb.WriteString(fmt.Sprintf("%s %s", v.Name, v.Regex))
	return sb.String()
}

func (v *TextFSMValue) processScalarValue(newval string) {
	var finalval interface{} = nil
	if FindIndex(v.Options, "List") >= 0 {
		// If the value is 'List', add the new value to the current value.
		if v.curval == nil {
			if FindIndex(v.Options, "Filldown") >= 0 && v.filldown_value != nil {
				// curval is null. But there is a filldown value. Append to filldown value
				finalval = append(v.filldown_value.([]string), newval)
			} else {
				finalval = make([]string, 0)
				finalval = append(finalval.([]string), newval)
			}
		} else {
			finalval = append(v.curval.([]string), newval)
		}
	} else {
		finalval = newval
	}
	if FindIndex(v.Options, "Filldown") >= 0 {
		// If there is Filldown present, Remember the new value as filldown value
		if finalval == nil {
			finalval = v.filldown_value
		} else {
			v.filldown_value = finalval
		}
	}
	v.curval = finalval
}

func (v *TextFSMValue) processMapValue(newval map[string]string) {
	newmap := make(map[string]string)
	var_names, err := GetGroupNames(v.Regex)
	if err != nil {
		panic(err)
	}
	for _, name := range var_names {
		newmap[name] = newval[name]
	}
	var finalval interface{} = newmap
	if FindIndex(v.Options, "List") >= 0 {
		// If the value is 'List', add the new value to the current value.
		if newval != nil && len(newval) > 0 {
			if v.curval == nil {
				if FindIndex(v.Options, "Filldown") >= 0 && v.filldown_value != nil {
					// curval is null. But there is a filldown value. Append to filldown value
					finalval = append(v.filldown_value.([]map[string]string), newmap)
				} else {
					finalval = make([]map[string]string, 0)
					finalval = append(finalval.([]map[string]string), newmap)
				}
			} else {
				finalval = append(v.curval.([]map[string]string), newmap)
			}
		}
	}
	if FindIndex(v.Options, "Filldown") >= 0 {
		// If there is Filldown present, Remember the new value as filldown value
		if finalval == nil {
			finalval = v.filldown_value
		} else {
			v.filldown_value = finalval
		}
	}
	v.curval = finalval
}

func (v *TextFSMValue) onAppendRecord() ON_RECORD_TYPE {
	if FindIndex(v.Options, "Required") >= 0 {
		if v.isEmptyValue(v.curval) {
			if FindIndex(v.Options, "Filldown") >= 0 {
				if v.isEmptyValue(v.filldown_value) {
					return SKIP_RECORD
				} else {
					return CONTINUE
				}
			}
			return SKIP_RECORD
		}
	}
	return CONTINUE
}

func (v *TextFSMValue) clearValue(all bool) {
	v.curval = nil
	if all && FindIndex(v.Options, "Filldown") >= 0 {
		v.filldown_value = nil
	}
}

func (v *TextFSMValue) getFinalValue() interface{} {
	if v.isEmptyValue(v.curval) && FindIndex(v.Options, "Filldown") >= 0 {
		return v.getFinalValueInternal(v.filldown_value)
	}
	return v.getFinalValueInternal(v.curval)
}
func (v *TextFSMValue) getFinalValueInternal(val interface{}) interface{} {
	if val == nil {
		if idx := FindIndex(v.Options, "List"); idx >= 0 {
			if strings.Contains(v.Regex, "(?P") {
				// If the regex contains (?P
				// ex: Value List ((?P<name>\w+)\s+(?P<age>\d+))
				// This will be an array of maps.
				return make([]map[string]string, 0)
			}
			// Else, it will be an array of strings
			return make([]string, 0)
		} else if strings.Contains(v.Regex, "(?P") {
			return make(map[string]string)
		} else {
			return ""
		}
	}
	return val
}

func (v *TextFSMValue) isEmptyValue(val interface{}) bool {
	if val == nil {
		return true
	}
	switch val.(type) {
	case string:
		return val.(string) == ""
	case []string:
		return len(val.([]string)) == 0
	case map[string]string:
		return len(val.(map[string]string)) == 0
	case []map[string]string:
		return len(val.([]map[string]string)) == 0
	default:
		panic(fmt.Sprintf("Unknown data type %v for %s", reflect.TypeOf(val), v.Name))
	}

}
