package clab

import (
	"fmt"

	"github.com/srl-labs/containerlab/utils"
)

// ProcessTopoPath takes a topology path, which might be the path to a directory or a file
// or stdin or a URL and returns the topology file name if found.
func ProcessTopoPath(path string, tmpDir string) (string, error) {
	var file string
	var err error

	switch {
	case path == "-" || path == "stdin":
		file, err = readFromStdin(tmpDir)
		if err != nil {
			return "", err
		}
	// if the path is not a local file and a URL, download the file and store it in the tmp dir
	case !utils.FileOrDirExists(path) && utils.IsHttpURL(path, true):
		file, err = downloadTopoFile(path, tmpDir)
		if err != nil {
			return "", err
		}

	case path == "":
		return "", fmt.Errorf("provide a path to the clab topology file")

	default:
		file, err = FindTopoFileByPath(path)
		if err != nil {
			return "", err
		}
	}
	return file, nil
}
