package gotextfsm

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

type TextFSM struct {
	COMMENT_RE         *regexp.Regexp
	STATE_RE           *regexp.Regexp
	MAX_STATE_NAME_LEN int
	Values             map[string]TextFSMValue
	States             map[string]TextFSMState
	line_num           int
}

// Parses the string passed, into a TextFSM structure.
// 	Args:
//		input string: Valid template as a string.
//	Retruns:
//		error if there is any error. nil otherwise
func (t *TextFSM) ParseString(input string) error {
	return t.ParseReader(strings.NewReader(input))
}

func (t *TextFSM) ParseReader(reader *strings.Reader) error {
	return t.ParseScanner(bufio.NewScanner(reader))
}

func (t *TextFSM) ParseScanner(scanner *bufio.Scanner) error {
	t.COMMENT_RE = regexp.MustCompile(`^\s*#`)
	t.STATE_RE = regexp.MustCompile(`^(\w+)$`)
	t.MAX_STATE_NAME_LEN = 48
	t.line_num = 0
	err := t.parseFSMVariables(scanner)
	if err != nil {
		return err
	}
	t.States = make(map[string]TextFSMState)
	for {
		done, err := t.parseFSMStates(scanner)
		if err != nil {
			return err
		}
		if done {
			break
		}
	}
	err = t.validateFSM()
	if err != nil {
		return err
	}
	return nil
}

// Extracts Variables from start of template file.
//     Values are expected as a contiguous block at the head of the file.
//     These will be line separated from the State definitions that follow.
//     Args:
//       scanner: Scanner to read through lines
//	   Returns:
//       returns error if there is any error while parsing. nil otherwise.
func (t *TextFSM) parseFSMVariables(scanner *bufio.Scanner) error {
	t.Values = make(map[string]TextFSMValue)
	t.line_num = 0
	for {
		t.line_num++
		line_present := scanner.Scan()
		if !line_present {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("%d Line: Scanner Error %s", t.line_num, err)
			}
			if t.line_num == 1 {
				return fmt.Errorf("Null template.")
			}
			return fmt.Errorf("No State definition found")
		}
		line := scanner.Text()
		line = TrimRightSpace(line)
		// Blank line signifies end of Value definitions.
		if line == "" {
			return nil
		}
		// Skip commented lines.
		if t.COMMENT_RE.MatchString(line) {
			continue
		}
		if strings.HasPrefix(line, "Value ") {
			value := TextFSMValue{}
			err := value.Parse(line, t.line_num)
			if err != nil {
				return err
			}
			t.Values[value.Name] = value
		} else if len(t.Values) == 0 {
			return fmt.Errorf("No Value definitions found.")
		} else {
			return fmt.Errorf("Expected blank line after last Value entry. Line: %d.", t.line_num)
		}
	}
}

// parseFSMStates extracts State and associated Rules from body of template file.
// After the Value definitions the remainder of the template is
// state definitions. The routine is expected to be called iteratively
// until no more states remain - indicated by returning None.
// The routine checks that the state names are a well formed string, do
// not clash with reserved names and are unique.
// Args:
//   scanner: Scanner to read lines from
// Returns:
//		done bool: true if there are no lines left in the file. false if there are lines left to parse
//      Error if there is any error. nil otherwise
func (t *TextFSM) parseFSMStates(scanner *bufio.Scanner) (done bool, err error) {
	for {
		t.line_num++
		line_present := scanner.Scan()
		if !line_present {
			if err := scanner.Err(); err != nil {
				return true, fmt.Errorf("%d Line: Scanner Error %s", t.line_num, err)
			}
			if len(t.States) == 0 {
				return true, fmt.Errorf("No State definition found")
			}
			return true, nil
		}
		line := scanner.Text()
		line = TrimRightSpace(line)
		if line == "" || t.COMMENT_RE.MatchString(line) {
			continue
		}
		// First line is state definition
		if !t.STATE_RE.MatchString(line) {
			return false, fmt.Errorf("%d Line: Invalid state name '%s'", t.line_num, line)
		}
		if len(line) > t.MAX_STATE_NAME_LEN {
			return false, fmt.Errorf("%d Line: state name too long. Should be < %d chars", t.line_num, len(line))
		}
		if FindIndex(LINE_OPERATORS, line) >= 0 || FindIndex(RECORD_OPERATORS, line) >= 0 {
			return false, fmt.Errorf("%d Line: state '%s' can not be a keyword", t.line_num, line)
		}
		if _, exists := t.States[line]; exists {
			return false, fmt.Errorf("%d Line: Duplicate state name '%s'", t.line_num, line)
		}
		state := TextFSMState{name: line, fsm: t}
		done, err = state.parseFSMRules(scanner)
		if err == nil {
			state.fsm.States[line] = state
		}
		return done, err
	}
}
func (t *TextFSMState) parseFSMRules(scanner *bufio.Scanner) (done bool, err error) {
	t.rules = make([]TextFSMRule, 0)
	for {
		t.fsm.line_num++
		line_present := scanner.Scan()
		if !line_present {
			if err := scanner.Err(); err != nil {
				return true, fmt.Errorf("%d Line: Scanner Error %s", t.fsm.line_num, err)
			}
			// Looks like a state with no rules is fine?
			// if len(t.rules) == 0 {
			// 	return true, fmt.Errorf("No Rule definition found")
			// }
			return true, nil
		}
		line := scanner.Text()
		line = TrimRightSpace(line)
		// Empty line indicates the end of state
		if line == "" {
			return false, nil
		}
		if t.fsm.COMMENT_RE.MatchString(line) {
			continue
		}
		valid := false
		for _, prefix := range []string{" ^", "  ^", "\t^"} {
			if strings.HasPrefix(line, prefix) {
				valid = true
				break
			}
		}
		if !valid {
			return false, fmt.Errorf("%d Line: Missing white space or carat ('^') before rule.", t.fsm.line_num)
		}
		rule := TextFSMRule{}
		varmap := make(map[string]interface{})
		for key, val := range t.fsm.Values {
			varmap[key] = val.Template
		}
		err = rule.Parse(line, t.fsm.line_num, varmap)
		if err != nil {
			return false, err
		}
		t.rules = append(t.rules, rule)
	}
}

// Checks state names and destinations for validity.
// Each destination state must exist, be a valid name and
// not be a reserved name.
// There must be a 'Start' state and if 'EOF' or 'End' states are specified,
// they must be empty.
// Returns:
//   error if the FSM is invalid
func (t *TextFSM) validateFSM() error {
	// Must have 'Start' state.
	if _, exists := t.States["Start"]; !exists {
		return fmt.Errorf("Missing state 'Start'.")
	}
	// 'End/EOF' state (if specified) must be empty.
	if state, exists := t.States["End"]; exists {
		if state.rules != nil && len(state.rules) > 0 {
			return fmt.Errorf("Non-Empty 'End' state.")
		} else {
			// Remove 'End' state.
			delete(t.States, "End")
		}
	}
	if state, exists := t.States["EOF"]; exists {
		if state.rules != nil && len(state.rules) > 0 {
			return fmt.Errorf("Non-Empty 'EOF' state.")
		}
	}
	// Ensure jump states are all valid.
	for name, state := range t.States {
		for _, rule := range state.rules {
			if rule.LineOp == "Error" {
				continue
			}
			if rule.NewState == "" || rule.NewState == "End" || rule.NewState == "EOF" {
				continue
			}
			if _, exists := t.States[rule.NewState]; !exists {
				return fmt.Errorf("State '%s' not found, referenced in state '%s'", rule.NewState, name)
			}
		}
	}
	return nil
}
