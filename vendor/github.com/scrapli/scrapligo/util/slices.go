package util

// StringSliceContains checks if the string slice ss contains the substring s.
func StringSliceContains(ss []string, s string) bool {
	for _, sss := range ss {
		if sss == s {
			return true
		}
	}

	return false
}
