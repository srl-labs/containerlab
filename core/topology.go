package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// FindTopoFileByPath takes a topology path, which might be the path to a directory
// and returns the topology file name if found.
func FindTopoFileByPath(path string) (string, error) {
	finfo, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	// by default we assume the path points to a clab file
	file := path

	// we might have gotten a dirname
	// lets try to find a single *.clab.y*ml
	if finfo.IsDir() {
		matches, err := filepath.Glob(filepath.Join(path, "*.clab.y*ml"))
		if err != nil {
			return "", err
		}

		switch len(matches) {
		case 1:
			// single file found, using it
			file = matches[0]
		case 0:
			// no files found
			return "", fmt.Errorf("no topology files found in directory %q", path)
		default:
			// multiple files found
			var filenames []string
			// extract just filename -> no path
			for _, match := range matches {
				filenames = append(filenames, filepath.Base(match))
			}

			return "", fmt.Errorf(
				"found multiple topology definitions [ %s ] in a given directory %q. "+
					"Provide the specific filename",
				strings.Join(filenames, ", "),
				path,
			)
		}
	}

	return file, nil
}

func downloadTopoFile(url, tempDir string) (string, error) {
	tmpFile, err := os.CreateTemp(tempDir, "topo-*.clab.yml")
	if err != nil {
		return "", err
	}

	err = clabutils.CopyFile(context.Background(), url, tmpFile.Name(),
		clabconstants.PermissionsFileDefault)

	return tmpFile.Name(), err
}

// readFromStdin reads the topology file from stdin
// creates a temp file with topology contents
// and returns a path to the temp file.
func readFromStdin(tempDir string) (string, error) {
	tmpFile, err := os.CreateTemp(tempDir, "topo-*.clab.yml")
	if err != nil {
		return "", err
	}

	_, err = tmpFile.ReadFrom(os.Stdin)
	if err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}
