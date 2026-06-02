package sros

import (
	"fmt"
	"regexp"
)

// getPrompt returns the prompt value from a string blob containing the prompt.
// The s is the output of the "environment show | grep -A 2 prompt" command.
func getPrompt(s string) (string, error) {
	re := regexp.MustCompile(`value\s+=\s+"(.+)"`)
	v := re.FindStringSubmatch(s)

	if len(v) != 2 {
		return "", fmt.Errorf("failed to parse prompt from string: %s", s)
	}

	return v[1], nil
}
