package gotextfsm

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// Dict contains a slice of maps. Each element in the slice holds the value of a record.
// Each record is represented as map of (name,value)
//
// Note that type of value is interface{}. But the concrete type is either 'string' or '[]string'
type ParserOutput struct {
	Dict           []map[string]interface{}
	line_num       int
	cur_state_name string
}

func (t *ParserOutput) Reset(fsm TextFSM) {
	t.clearRecord(fsm, true)
	t.cur_state_name = "Start"
	t.Dict = make([]map[string]interface{}, 0)
}

// ParseTextString passes CLI output (provided as string) through FSM and
//     Args:
//       text: (string), Text to parse with embedded newlines.
//		 fsm: (TextFSM), TextFSM object as a result of parsing the text fsm template
//       eof: (bool), Set to False if we are parsing only part of the file.
//             Suppresses triggering EOF state.
//     Returns:
//       error if there is any error in parsing
func (t *ParserOutput) ParseTextString(text string, fsm TextFSM, eof bool) error {
	return t.ParseTextReader(strings.NewReader(text), fsm, eof)
}

func (t *ParserOutput) ParseTextReader(reader *strings.Reader, fsm TextFSM, eof bool) error {
	return t.ParseTextScanner(bufio.NewScanner(reader), fsm, eof)
}

func (t *ParserOutput) ParseTextScanner(scanner *bufio.Scanner, fsm TextFSM, eof bool) error {
	t.line_num = 0
	if t.cur_state_name == "" {
		t.cur_state_name = "Start"
	}
	if t.Dict == nil {
		t.Dict = make([]map[string]interface{}, 0)
	}
	for {
		t.line_num++
		line_present := scanner.Scan()
		if !line_present {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("%d Line: Scanner Error %s", t.line_num, err)
			}
			break
		}
		line := scanner.Text()
		err := t.checkLine(line, fsm)
		if err != nil {
			return err
		}
		if t.cur_state_name == "End" || t.cur_state_name == "EOF" {
			break
		}
	}
	_, eof_exists := fsm.States["EOF"]
	if t.cur_state_name != "End" && (!eof_exists) && eof {
		// Implicit EOF performs Next.Record operation.
		// Suppressed if Null EOF state is instantiated.
		t.appendRecord(fsm)
	}
	return nil
}

// checkLine passes the line through each rule until a match is made.
// If the value regex contains nested match groups in the form (?P<name>regex),
//     In case of List type with nested match groups
//        instead of adding a string to the list, we add a dictionary of the groups.
//     Other value types with nested match groups,
//         the value is set as 'map[string]string' instead of a 'string'
//     Eg.
//     Value List ((?P<name>\w+)\s+(?P<age>\d+)) would create results like:
//         [{'name': 'Bob', 'age': 32}]
//     Do not give nested groups the same name as other values in the template.
//     Nested regexps more than 2 levels are not supported currently
//     Args:
//       line: A string, the current input line.
//		 fsm: TextFSM Object
func (t *ParserOutput) checkLine(line string, fsm TextFSM) error {
	// fmt.Printf("Looking at line '%s'\n", line)
	state, exists := fsm.States[t.cur_state_name]
	if !exists {
		// Should never happen for a proper TextFSM
		panic(fmt.Sprintf("Unknown State %s", t.cur_state_name))
	}
	for _, rule := range state.rules {
		varmap := GetNamedMatches(regexp.MustCompile(rule.Regex), line)
		if varmap != nil {
			// fmt.Printf("Line '%s'. Regex: '%s' varmap: '%v'\n", line, rule.Regex, varmap)
			for key, val := range varmap {
				valobj, exists := fsm.Values[key]
				if !exists {
					// This may happen in case of nested match groups.
					// There will be no TextFSMValue with the names inside the the nested match groups.
					continue
				}
				if strings.Contains(valobj.Regex, "(?P") {
					valobj.processMapValue(varmap)
				} else {
					valobj.processScalarValue(val)
				}
				if FindIndex(valobj.Options, "Fillup") >= 0 && valobj.curval != nil && t.Dict != nil {
					for i := len(t.Dict) - 1; i >= 0; i-- {
						if valobj.isEmptyValue(t.Dict[i][key]) {
							t.Dict[i][key] = valobj.curval
						} else {
							break
						}
					}
				}
				// For some reason, modifying curval in valobj using processValue is not reflecting.
				// Setting it back to fsm.Values works. Need to understand this further
				fsm.Values[key] = valobj
			}
			output, err := t.handleOperations(rule, fsm, line)
			if err != nil {
				return err
			}
			if output {
				if rule.NewState != "" {
					t.cur_state_name = rule.NewState
				}
				break
			}
		}
	}
	// fmt.Printf("After Line: '%s: ' current state: '%s'\n", line, t.cur_state_name)

	// for name, varobj := range fsm.Values {
	// 	fmt.Printf(" %s: curval '%v', filldownval '%v', ", name, varobj.curval, varobj.filldown_value)
	// }
	// fmt.Printf("\n")
	return nil
}

// appendRecord adds current record to result if well formed.
func (t *ParserOutput) appendRecord(fsm TextFSM) {
	newmap := make(map[string]interface{})
	any_value := false
	for name, value := range fsm.Values {
		ret := value.onAppendRecord()
		switch ret {
		case SKIP_RECORD:
			t.clearRecord(fsm, false)
			return
		case SKIP_VALUE:
			newmap[name] = nil
		case CONTINUE:
			newmap[name] = value.getFinalValue()
			if !value.isEmptyValue(newmap[name]) {
				any_value = true
			}
		}
	}
	// If no Values in template or whole record is empty then don't output.
	if any_value {
		t.Dict = append(t.Dict, newmap)
	}
	t.clearRecord(fsm, false)
}

// handleOperation handles Operators on the data record.
//
// Operators come in two parts and are a '.' separated pair:
//   Operators that effect the input line or the current state (line_op).
// 	'Next'      Get next input line and restart parsing (default).
// 	'Continue'  Keep current input line and continue resume parsing.
// 	'Error'     Unrecoverable input discard result and raise Error.
//
//
//   Operators that affect the record being built for output (record_op).
// 	'NoRecord'  Does nothing (default)
// 	'Record'    Adds the current record to the result.
// 	'Clear'     Clears non-Filldown data from the record.
// 	'Clearall'  Clears all data from the record.
//
// Args:
//   rule: FSMRule object.
//   line: A string, the current input line.
// Returns:
//   True if state machine should restart state with new line.
//   error: If Error state is encountered.
func (t *ParserOutput) handleOperations(rule TextFSMRule, fsm TextFSM, line string) (output bool, err error) {
	if rule.RecordOp == "Record" {
		t.appendRecord(fsm)
	}
	if rule.RecordOp == "Clear" {
		t.clearRecord(fsm, false)
	}
	if rule.RecordOp == "Clearall" {
		t.clearRecord(fsm, true)
	}
	if rule.LineOp == "Error" {
		if rule.NewState != "" {
			return false, fmt.Errorf("Error: %s. Rule Line: %d. Input Line: %s.", rule.NewState, rule.LineNum, line)
		} else {
			return false, fmt.Errorf("State Error raised. Rule Line: %d. Input Line: %s", rule.LineNum, line)
		}
	} else if rule.LineOp == "Continue" {
		return false, nil
	}
	return true, nil
}

func (t *ParserOutput) clearRecord(fsm TextFSM, all bool) {
	for name, value := range fsm.Values {
		value.clearValue(all)
		// For some reason, modifying curval in valobj using processValue is not reflecting.
		// Setting it back to fsm.Values works. Need to understand this further
		fsm.Values[name] = value
	}
}
