package util

import (
	"fmt"
	"time"
)

// CreateCandidateConfigName returns a name to use as the candidate configuration name -- if there
// is a provided candidate configuration name we prefer to use that. If the timestamp bool is true
// we append a unix timestamp to the candidate configuration name. With no name provided and
// timestamp false we simply use 'scrapli_cfg_candidate'.
func CreateCandidateConfigName(s string, timestamp bool) string {
	if s != "" {
		if timestamp {
			return fmt.Sprintf("%s_%d", s, time.Now().Unix())
		}

		return s
	}

	if timestamp {
		return fmt.Sprintf("scrapli_cfg_candidate_%d", time.Now().Unix())
	}

	return "scrapli_cfg_candidate"
}
