package utils

import (
	"fmt"
	"regexp"
)

func GetRegexpCaptureGroups(r *regexp.Regexp, search string) (map[string]string, error) {
	matches := r.FindStringSubmatch(search)
	if len(matches) == 0 {
		return nil, fmt.Errorf("%q does not match regexp %q, no match", search, r)
	}

	captureGroups := make(map[string]string)
	for i, name := range r.SubexpNames() {
		if i != 0 && name != "" {
			captureGroups[name] = matches[i]
		}
	}

	return captureGroups, nil
}
