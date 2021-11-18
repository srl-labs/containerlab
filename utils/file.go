// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func FileExists(filename string) bool {
	f, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !f.IsDir()
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, copy the file contents from src to dst.
// mode is the desired target file permissions, e.g. "0644"
func CopyFile(src, dst string, mode os.FileMode) (err error) {
	var sfi os.FileInfo
	if isHTTP := (strings.HasPrefix("http://", src) || strings.HasPrefix("https://", src)); !isHTTP {
		fmt.Println("here011")
		sfi, err = os.Stat(src)
		if err != nil {
			return err
		}
		if !sfi.Mode().IsRegular() {
			// cannot copy non-regular files (e.g., directories,
			// symlinks, devices, etc.)
			return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
		}
	}

	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}

		if sfi != nil && os.SameFile(sfi, dfi) {
			return
		}
	}

	return CopyFileContents(src, dst, mode)
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
// src can be an http(s) URL as well
func CopyFileContents(src, dst string, mode os.FileMode) (err error) {
	var in io.ReadCloser
	if isHTTP := (strings.HasPrefix("http://", src) || strings.HasPrefix("https://", src)); isHTTP {
		resp, err := http.Get(src)
		if err != nil {
			return fmt.Errorf("failed to fetch HTTP resource by the path %s: %v", src, err)
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
		return
	}

	err = out.Chmod(mode)
	if err != nil {
		return
	}

	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return
	}

	err = out.Sync()

	return
}

// CreateFile writes content to a file by path `file`
func CreateFile(file, content string) error {
	var f *os.File
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(content + "\n"); err != nil {
		return err
	}

	return nil
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
		return nil, fmt.Errorf("file %s does not exist", file)
	}

	// read and return file content
	b, err := ioutil.ReadFile(file)
	return b, err
}
