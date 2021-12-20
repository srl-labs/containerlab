// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

var errNonRegularFile = errors.New("non-regular file")
var errFileNotExist = errors.New("file does not exist")
var errHTTPFetch = errors.New("failed to fetch http(s) resource")

// FileExists returns true if a file referenced by filename exists
func FileExists(filename string) bool {
	f, err := os.Stat(filename)
	if os.IsNotExist(err) {
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
			return fmt.Errorf("file copy failed: destination file %s (%q): %w", dfi.Name(), dfi.Mode().String(), errNonRegularFile)
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
	defer in.Close()

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
	if err == nil {
		defer f.Close()
		_, err = f.WriteString(content + "\n")
	}

	return err
}

// CreateDirectory creates a directory by a path with a mode/permission specified by perm.
// If directory exists, the function does not do anything.
func CreateDirectory(path string, perm os.FileMode) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.MkdirAll(path, perm)
	}
}

func ReadFileContent(file string) ([]byte, error) {
	// check file exists
	if !FileExists(file) {
		return nil, fmt.Errorf("%w: %s", errFileNotExist, file)
	}

	// read and return file content
	b, err := ioutil.ReadFile(file)

	return b, err
}
