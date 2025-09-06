package transport

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
)

// SSHKind is an interface to implement kind specific methods for transactions and prompt checking.
type SSHKind interface {
	// Start a config transaction
	ConfigStart(s *SSHTransport, transaction bool) error
	// Commit a config transaction
	ConfigCommit(s *SSHTransport) (*SSHReply, error)
	// Prompt parsing function
	//
	// This function receives string, split by the delimiter and should ensure this is a valid prompt
	// Valid prompt, strip the prompt from the result and add it to the prompt in SSHReply
	//
	// A default implementation is promptParseNoSpaces, which simply ensures there are
	// no spaces between the start of the line and the #
	PromptParse(s *SSHTransport, in *string) *SSHReply
}

// VrSrosSSHKind implements SShKind.
type VrSrosSSHKind struct{}

func (*VrSrosSSHKind) ConfigStart(s *SSHTransport, transaction bool) error { // skipcq: RVV-A0005
	s.PromptChar = "#" // ensure it's '#'

	r := s.Run("/environment more false", 5)
	if r.result != "" {
		log.Warnf("%s Are you in MD-Mode?%s", s.Target, r.LogString(s.Target, true, false))
	}

	if transaction {
		s.Run("/configure global", 5).Info(s.Target)
		s.Run("discard", 1).Info(s.Target)
	}

	return nil
}

func (*VrSrosSSHKind) ConfigCommit(s *SSHTransport) (*SSHReply, error) {
	res := s.Run("commit", 10)
	if res.result != "" {
		return res, fmt.Errorf("could not commit %s", res.result)
	}

	return res, nil
}

func (*VrSrosSSHKind) PromptParse(s *SSHTransport, in *string) *SSHReply {
	// SROS MD-CLI \r...prompt
	r := strings.LastIndex(*in, "\r\n\r\n")
	if r > 0 {
		return &SSHReply{
			result: (*in)[:r],
			prompt: (*in)[r+4:] + s.PromptChar,
		}
	}

	return nil
}

// SrosSSHKind implements SShKind.
type SrosSSHKind struct{}

func (*SrosSSHKind) ConfigStart(s *SSHTransport, transaction bool) error { // skipcq: RVV-A0005
	s.PromptChar = "#" // ensure it's '#'

	r := s.Run("/environment more false", 5)
	if r.result != "" {
		log.Warnf("%s Are you in MD-Mode?%s", s.Target, r.LogString(s.Target, true, false))
	}

	if transaction {
		s.Run("/configure global", 5).Info(s.Target)
		s.Run("discard", 1).Info(s.Target)
	}

	return nil
}

func (*SrosSSHKind) ConfigCommit(s *SSHTransport) (*SSHReply, error) {
	res := s.Run("commit", 10)
	if res.result != "" {
		return res, fmt.Errorf("could not commit %s", res.result)
	}

	return res, nil
}

func (*SrosSSHKind) PromptParse(s *SSHTransport, in *string) *SSHReply {
	// SROS MD-CLI \r...prompt
	r := strings.LastIndex(*in, "\r\n\r\n")
	if r > 0 {
		return &SSHReply{
			result: (*in)[:r],
			prompt: (*in)[r+4:] + s.PromptChar,
		}
	}

	return nil
}

// SrlSSHKind implements SShKind.
type SrlSSHKind struct{}

func (*SrlSSHKind) ConfigStart(s *SSHTransport, transaction bool) error { // skipcq: RVV-A0005
	s.PromptChar = "#" // ensure it's '#'
	if transaction {
		r0 := s.Run("enter candidate private", 5)
		r1 := s.Run("discard stay", 2)

		if !strings.Contains(r1.result, "Nothing to discard") {
			r0.result += "; " + r1.result
			r0.command += "; " + r1.command
		}

		r0.Info(s.Target)
	}

	return nil
}

func (*SrlSSHKind) ConfigCommit(s *SSHTransport) (*SSHReply, error) {
	r := s.Run("commit now", 10)

	if strings.Contains(r.result, "All changes have been committed") {
		r.result = ""
	} else {
		return r, fmt.Errorf("could not commit %s", r.result)
	}

	return r, nil
}

func (*SrlSSHKind) PromptParse(s *SSHTransport, in *string) *SSHReply {
	return promptParseNoSpaces(in, s.PromptChar, 2)
}

// This is a helper function to parse the prompt, and can be used by SSHKind's ParsePrompt
// Used in SRL today.
func promptParseNoSpaces(in *string, promptChar string, lines int) *SSHReply {
	n := strings.LastIndex(*in, "\n")
	if n < 0 {
		return nil
	}

	if strings.Contains((*in)[n:], " ") {
		return nil
	}

	if lines > 1 {
		// Add another line to the prompt
		res := (*in)[:n]
		n = strings.LastIndex(res, "\n")
	}

	if n < 0 {
		n = 0
	}

	return &SSHReply{
		result: (*in)[:n],
		prompt: (*in)[n:] + promptChar,
	}
}
