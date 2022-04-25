package srl

import "regexp"

// SrlVersion represents an sr linux version as a set of fields
type SrlVersion struct {
	major  string
	minor  string
	patch  string
	build  string
	commit string
}

func (*srl) parseVersionString(s string) *SrlVersion {
	re, _ := regexp.Compile(`v(\d{1,3})\.(\d{1,2})\.(\d{1,3})\-(\d{1,4})\-(\S+)`)

	v := re.FindStringSubmatch(s)
	// 6 matches must be returned if all goes well
	if len(v) != 6 {
		// return all zeroes if failed to parse
		return &SrlVersion{"0", "0", "0", "0", "0"}
	}

	return &SrlVersion{v[1], v[2], v[3], v[4], v[5]}
}
