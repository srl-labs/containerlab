package utils

import (
  "strings"
  "errors"
  "net/http"
  "log"
  "io"
  "os"
)

// write a methd to replace "/blob" with ""


func GetRawURL(url string) string {
	
	raw:=strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
	return strings.Replace(raw, "/blob", "", 1)
}

func IsGitHubURL(url string) bool {
	return strings.Contains(url, "github")

}

func GetFileName(url string) string {
	split := strings.Split(url, "/")
	return split[len(split)-1]
}

func CheckSuffix(url string) error {
	// check if topo ends with either .yml or .yaml
	if !strings.HasSuffix(url, ".yml") && !strings.HasSuffix(url, ".yaml") {
		return errors.New("valid URL passed in as topology file, but does not end with .yml or .yaml, endpoint must be an actual topology file")
	}
	return nil
}

func DownloadFile(url string, ouputFileName string) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 200 {
		log.Fatal("URL does not exist")
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(ouputFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Fatal(err)
	}
}
