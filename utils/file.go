// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	errNonRegularFile = errors.New("non-regular file")
	errHTTPFetch      = errors.New("failed to fetch http(s) resource")
)

// FileExists returns true if a file referenced by filename exists & accessible.
func FileExists(filename string) bool {
	f, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !f.IsDir()
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, copy the file contents from src to dst.
// mode is the desired target file permissions, e.g. "0644".
func CopyFile(src, dst string, mode os.FileMode) (err error) {
	var sfi os.FileInfo
	if isHTTP := (strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")); !isHTTP {
		sfi, err = os.Stat(src)
		if err != nil {
			return err
		}

		if !sfi.Mode().IsRegular() {
			// cannot copy non-regular files (e.g., directories,
			// symlinks, devices, etc.)
			return fmt.Errorf("file copy failed: source file %s (%q): %w", sfi.Name(), sfi.Mode().String(), errNonRegularFile)
		}
	}

	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("file copy failed: destination file %s (%q): %w",
				dfi.Name(), dfi.Mode().String(), errNonRegularFile)
		}

		if sfi != nil && os.SameFile(sfi, dfi) {
			return nil
		}
	}

	return CopyFileContents(src, dst, mode)
}

// CopyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
// src can be an http(s) URL as well.
func CopyFileContents(src, dst string, mode os.FileMode) (err error) {
	var in io.ReadCloser

	if isHTTP := (strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")); isHTTP {
		resp, err := http.Get(src)
		if err != nil || resp.StatusCode != 200 {
			return fmt.Errorf("%w: %s", errHTTPFetch, src)
		}

		in = resp.Body
	} else {
		in, err = os.Open(src)
		if err != nil {
			return err
		}
	}
	defer in.Close() // skipcq: GO-S2307

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	err = out.Chmod(mode)
	if err != nil {
		return err
	}

	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	err = out.Sync()

	return err
}

// CreateFile writes content to a file by path `file`.
func CreateFile(file, content string) (err error) {
	var f *os.File

	f, err = os.Create(file)
	if err != nil {
		return err
	}

	_, err = f.WriteString(content + "\n")
	if err != nil {
		return err
	}

	return f.Close()
}

// CreateDirectory creates a directory by a path with a mode/permission specified by perm.
// If directory exists, the function does not do anything.
func CreateDirectory(path string, perm os.FileMode) {
	err := os.MkdirAll(path, perm)
	if err != nil {
		log.Debugf("error while creating a directory path %v: %v", path, err)
	}
}

func ReadFileContent(file string) ([]byte, error) {
	// try to read and return file content, or return an error
	b, err := os.ReadFile(file)
	return b, err
}

// ExpandHome expands `~` char in the path to home path of a current user in provided path p.
func ExpandHome(p string) string {
	userPath, _ := os.UserHomeDir()

	p = strings.Replace(p, "~", userPath, 1)

	return p
}

// ResolvePath resolves a string path by expanding `~` to home dir
// or resolving a relative path by joining it with the base path.
func ResolvePath(p, base string) string {
	if p == "" {
		return p
	}

	switch {
	// resolve ~/ path
	case p[0] == '~':
		p = ExpandHome(p)
	case p[0] == '/':
		return p
	default:
		// join relative path with the base path
		p = filepath.Join(base, p)
	}
	return p
}

const (
	CalcFilename_Undefined = "undefined"
)

// CalcFilename tries to deduce a filename from the given url
// returns "undefined" if facing issues
func CalcFilename(rawUrl string) string {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return CalcFilename_Undefined
	}
	pathElems := strings.Split(u.Path, "/")
	lastPathElem := pathElems[len(pathElems)-1]
	if len(lastPathElem) == 0 {
		return CalcFilename_Undefined
	}
	return lastPathElem
}
