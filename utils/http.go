package utils

import (
  "strings"
  "errors"
)


func GetRawURL(url string) string {
	if strings.Contains(url, "github.com") {
		raw:=strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
		return strings.Replace(raw, "/blob", "", 1)
	}
	return url
}

func IsGitHubURL(url string) bool {
	return strings.Contains(url, "github")

}

// required global variable for tests, otherwise comparison operator fails as error instances were not equal
var ErrInvalidSuffix = errors.New("valid URL passed in as topology file, but does not end with .yml or .yaml, endpoint must be an actual topology file")
func CheckSuffix(url string) error {
	// check if topo ends with either .yml or .yaml
	if !strings.HasSuffix(url, ".yml") && !strings.HasSuffix(url, ".yaml") {
		return ErrInvalidSuffix
	}
	return nil
}
