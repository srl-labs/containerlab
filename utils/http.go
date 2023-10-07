package utils

import (
  "strings"
  "errors"
  "net/http"
  "io"
  "os"
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

func GetFileName(url string) string {
	split := strings.Split(url, "/")
	return split[len(split)-1]
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

func DownloadFile(url string, ouputFileName string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("URL does not exist")
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(ouputFileName)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
