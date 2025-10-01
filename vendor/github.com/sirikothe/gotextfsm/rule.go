package gotextfsm

import (
	"fmt"
	"regexp"
	"strings"
)

type TextFSMRule struct {
	Regex    string
	Match    string
	LineOp   string
	RecordOp string
	NewState string
	LineNum  int
}

var LINE_OPERATORS = []string{"Continue", "Next", "Error"}
var RECORD_OPERATORS = []string{"Clear", "Clearall", "Record", "NoRecord"}

func (t *TextFSMRule) String() string {
	var sb strings.Builder
	sb.WriteString(" " + t.Match)
	if t.LineOp != "" {
		sb.WriteString(" -> " + t.LineOp)
		if t.RecordOp != "" {
			sb.WriteString("." + t.RecordOp)
		}
		if t.NewState != "" {
			sb.WriteString(" " + t.NewState)
		}
	} else {
		if t.RecordOp != "" {
			sb.WriteString(" -> " + t.RecordOp)
			if t.NewState != "" {
				sb.WriteString(" " + t.NewState)
			}
		} else if t.NewState != "" {
			sb.WriteString(" -> " + t.NewState)
		}
	}
	return sb.String()
}
func (r *TextFSMRule) Parse(line string, lineNum int, var_map map[string]interface{}) error {
	r.LineNum = lineNum
	// Implicit default is '(regexp) -> Next.NoRecord'
	MATCH_ACTION := regexp.MustCompile(`(?P<match>.*)(\s->(?P<action>.*))`)
	// Line operators.
	OPER_RE := regexp.MustCompile(`(?P<ln_op>Continue|Next|Error)`)
	// Record operators.
	RECORD_RE := regexp.MustCompile(`(?P<rec_op>Clear|Clearall|Record|NoRecord)`)
	// Line operator with optional record operator.
	OPER_RECORD_RE := regexp.MustCompile(fmt.Sprintf("(%s(%s%s)?)", OPER_RE, `\.`, RECORD_RE))
	// New State or 'Error' string.
	NEWSTATE_RE := regexp.MustCompile(`(?P<new_state>\w+|\".*\")`)
	// Compound operator (line and record) with optional new state.
	ACTION_RE := regexp.MustCompile(fmt.Sprintf("^%s%s(%s%s)?$", `\s+`, OPER_RECORD_RE, `\s+`, NEWSTATE_RE))
	// Record operator with optional new state.
	ACTION2_RE := regexp.MustCompile(fmt.Sprintf("^%s%s(%s%s)?$", `\s+`, RECORD_RE, `\s+`, NEWSTATE_RE))
	// Default operators with optional new state.
	ACTION3_RE := regexp.MustCompile(fmt.Sprintf("^(%s%s)?$", `\s+`, NEWSTATE_RE))
	line = strings.TrimSpace(line)
	if line == "" {
		return fmt.Errorf("Null data in FSMRule. Line: %d", r.LineNum)
	}
	// Is there '->' action present. ?
	matches := GetNamedMatches(MATCH_ACTION, line)
	if matches != nil {
		r.Match = matches["match"]
	} else {
		r.Match = line
	}
	if var_map != nil {
		regex, err := ExecutePythonTemplate(r.Match, var_map)
		if err != nil {
			return err
		}
		r.Regex = regex
	}
	if _, err := regexp.Compile(r.Regex); err != nil {
		return fmt.Errorf("Line %d: Invalid regular expression '%s'. Error: '%s'", r.LineNum, r.Regex, err.Error())
	}
	if _, err := regexp.Compile(r.Match); err != nil {
		return fmt.Errorf("Line %d: Invalid regular expression '%s'. Error: '%s'", r.LineNum, r.Match, err.Error())
	}
	action := matches["action"]
	m := GetNamedMatches(ACTION_RE, action)
	if m == nil {
		m = GetNamedMatches(ACTION2_RE, action)
	}
	if m == nil {
		m = GetNamedMatches(ACTION3_RE, action)
	}
	if m == nil {
		return fmt.Errorf("Badly formatted rule '%s'. Line: %d.", line, r.LineNum)
	}
	if ln_op, exists := m["ln_op"]; exists {
		r.LineOp = ln_op
	}
	if rec_op, exists := m["rec_op"]; exists {
		r.RecordOp = rec_op
	}
	if new_state, exists := m["new_state"]; exists {
		r.NewState = new_state
	}
	// Only 'Next' (or implicit 'Next') line operator can have a new_state.
	// But we allow error to have one as a warning message so we are left
	// checking that Continue does not.
	if r.LineOp == "Continue" && r.NewState != "" {
		return fmt.Errorf("Action '%s' with new state %s specified. Line: %d.", r.LineOp, r.NewState, r.LineNum)
	}
	// Check that an error message is present only with the 'Error' operator.
	if r.LineOp != "Error" && r.NewState != "" {
		if !regexp.MustCompile(`^\w+$`).MatchString(r.NewState) {
			return fmt.Errorf("Alphanumeric characters only in state names. Line: %d.", r.LineNum)
		}
	}
	return nil
}
