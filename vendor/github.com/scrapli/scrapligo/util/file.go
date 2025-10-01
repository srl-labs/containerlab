package util

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ResolveFilePath resolves the qualified path to file string f.
func ResolveFilePath(f string) (string, error) {
	_, err := os.Stat(f)
	if err == nil {
		return f, nil
	}

	// if didn't stat a fully qualified file, strip user dir (if exists) and then check there
	f = strings.TrimPrefix(f, "~/")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	f = fmt.Sprintf("%s/%s", homeDir, f)

	if _, err = os.Stat(f); err == nil {
		return f, nil
	}

	return "", ErrFileNotFoundError
}

// LoadFileLines convenience function to load a file and return slice of strings of lines in that
// file.
func LoadFileLines(f string) ([]string, error) {
	resolvedFile, err := ResolveFilePath(f)
	if err != nil {
		return []string{}, err
	}

	file, err := os.Open(resolvedFile) //nolint: gosec
	if err != nil {
		return []string{}, err
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	var lines []string

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, nil
}
