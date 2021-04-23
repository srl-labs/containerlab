package config

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// an interface to implement kind specific methods for transactions and prompt checking
type SshKind interface {
	// Start a config transaction
	ConfigStart(s *SshTransport, transaction bool) error
	// Commit a config transaction
	ConfigCommit(s *SshTransport) (*SshReply, error)
	// Prompt parsing function.
	// This function receives string, split by the delimiter and should ensure this is a valid prompt
	// Valid prompt, strip te prompt from the result and add it to the prompt in SshReply
	//
	// A defualt implementation is promptParseNoSpaces, which simply ensures there are
	// no spaces between the start of the line and the #
	PromptParse(s *SshTransport, in *string) *SshReply
}

// implements SShKind
type VrSrosSshKind struct{}

func (sk *VrSrosSshKind) ConfigStart(s *SshTransport, transaction bool) error {
	s.PromptChar = "#" // ensure it's '#'
	//s.debug = true
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
func (sk *VrSrosSshKind) ConfigCommit(s *SshTransport) (*SshReply, error) {
	res := s.Run("commit", 10)
	if res.result != "" {
		return res, fmt.Errorf("could not commit %s", res.result)
	}
	return res, nil
}

func (sk *VrSrosSshKind) PromptParse(s *SshTransport, in *string) *SshReply {
	// SROS MD-CLI \r...prompt
	r := strings.LastIndex(*in, "\r\n\r\n")
	if r > 0 {
		return &SshReply{
			result: (*in)[:r],
			prompt: (*in)[r+4:] + s.PromptChar,
		}
	}
	return nil
}

// implements SShKind
type SrlSshKind struct{}

func (sk *SrlSshKind) ConfigStart(s *SshTransport, transaction bool) error {
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
func (sk *SrlSshKind) ConfigCommit(s *SshTransport) (*SshReply, error) {
	r := s.Run("commit now", 10)
	if strings.Contains(r.result, "All changes have been committed") {
		r.result = ""
	} else {
		return r, fmt.Errorf("could not commit %s", r.result)
	}
	return r, nil
}
func (sk *SrlSshKind) PromptParse(s *SshTransport, in *string) *SshReply {
	return promptParseNoSpaces(in, s.PromptChar, 2)
}

// This is a helper funciton to parse the prompt, and can be used by SshKind's ParsePrompt
// Used in SRL today
func promptParseNoSpaces(in *string, promptChar string, lines int) *SshReply {
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
	return &SshReply{
		result: (*in)[:n],
		prompt: (*in)[n:] + promptChar,
	}
}
