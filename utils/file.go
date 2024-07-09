// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/steiler/acls"

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

// FileOrDirExists returns true if a file or dir referenced by path exists & accessible.
func FileOrDirExists(filename string) bool {
	f, err := os.Stat(filename)

	return err == nil && f != nil
}

// DirExists returns true if a dir referenced by path exists & accessible.
func DirExists(filename string) bool {
	f, err := os.Stat(filename)

	return err == nil && f != nil && f.IsDir()
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, copy the file contents from src to dst.
// mode is the desired target file permissions, e.g. "0644".
func CopyFile(src, dst string, mode os.FileMode) (err error) {
	var sfi os.FileInfo
	if !IsHttpURL(src, false) {
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
		if !errors.Is(err, fs.ErrNotExist) {
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

// IsHttpURL checks if the url is a downloadable HTTP URL.
// The allowSchemaless toggle when set to true will allow URLs without a schema
// such as "srlinux.dev/clab-srl". This is shortened notion that is used with
// "deploy -t <url>" only.
// Other callers of IsHttpURL should set the toggle to false.
func IsHttpURL(s string, allowSchemaless bool) bool {
	// '-' denotes stdin and not the URL
	if s == "-" {
		return false
	}

	//
	if !allowSchemaless && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}

	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}

	u, err := url.ParseRequestURI(s)

	return err == nil && u.Host != ""
}

// CopyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
// src can be an http(s) URL as well.
func CopyFileContents(src, dst string, mode os.FileMode) (err error) {
	var in io.ReadCloser

	if IsHttpURL(src, false) {
		client := NewHTTPClient()

		// download using client
		resp, err := client.Get(src)
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

	// create directories if needed, since we promise to create the file
	// if it doesn't exist
	err = os.MkdirAll(filepath.Dir(dst), 0750)
	if err != nil {
		return err
	}

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

	// add newline if missing
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	_, err = f.WriteString(content)
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
// When sudo is used, it expands to home dir of a sudo user.
func ExpandHome(p string) string {
	// current user home dir, used when sudo is not used
	// or when errors occur during sudo user lookup
	curUserHomeDir, _ := os.UserHomeDir()

	userId, isSet := os.LookupEnv("SUDO_UID")
	if !isSet {
		log.Debugf("SUDO_UID env var is not set, using current user home dir: %v", curUserHomeDir)
		p = strings.Replace(p, "~", curUserHomeDir, 1)
		return p
	}

	// lookup user to figure out Home Directory
	u, err := user.LookupId(userId)
	if err != nil {
		log.Debugf("error while looking up user by id using os/user.LookupId %v: %v", userId, err)
		// user.LookupId fails when ActiveDirectory is used, so we try to use getent command
		homedir := lookupUserHomeDirViaGetent(userId)
		if homedir != "" {
			log.Debugf("user home dir %v found using getent command", homedir)
			p = strings.Replace(p, "~", homedir, 1)
			return p
		}
		// fallback to current user home dir if getent command fails
		p = strings.Replace(p, "~", curUserHomeDir, 1)
		return p
	}

	p = strings.Replace(p, "~", u.HomeDir, 1)

	log.Debugf("user home dir %v found using os/user.LookupId", u.HomeDir)

	return p
}

// lookupUserHomeDirViaGetent looks up user's homedir by using `getent passwd` command.
// It is used as a fallback when os/user.LookupId fails, which seems to
// happen when ActiveDirectory is used.
func lookupUserHomeDirViaGetent(userId string) string {
	cmd := exec.Command("getent", "passwd", userId)
	out, err := cmd.Output()
	if err != nil {
		log.Debugf("error while looking up user by id using getent command %v: %v", userId, err)
		return ""
	}

	// output format is `username:x:uid:gid:comment:home:shell`
	// we need to extract home dir
	parts := strings.Split(string(out), ":")
	if len(parts) < 6 {
		log.Debugf("error while looking up user by id using getent command %v: unexpected output format", userId)
		return ""
	}

	return parts[5]
}

// ResolvePath resolves a string path by expanding `~` to home dir
// or resolving a relative path by joining it with the base path.
// When resolving `~` the function uses the home dir of a sudo user, so that -E sudo
// flag can be omitted.
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
	UndefinedFileName = "undefined"
)

// FilenameForURL extracts a filename from a given url
// returns "undefined" when unsuccessful.
func FilenameForURL(rawUrl string) string {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return UndefinedFileName
	}

	// try extracting the filename from "content-disposition" header
	if IsHttpURL(rawUrl, false) {
		resp, err := http.Head(rawUrl)
		if err != nil {
			return filepath.Base(u.Path)
		}
		if cd := resp.Header.Get("Content-Disposition"); cd != "" {
			if _, params, err := mime.ParseMediaType(cd); err == nil {
				return params["filename"]
			}
		}
	}
	return filepath.Base(u.Path)
}

// FileLines opens a file by the `path` and returns a slice of strings for each line
// excluding lines that start with `commentStr` or are empty.
func FileLines(path, commentStr string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %v: %w", path, err)
	}
	defer f.Close() // skipcq: GO-S2307

	var lines []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip lines that start with comment char
		if strings.HasPrefix(line, commentStr) || line == "" {
			continue
		}

		lines = append(lines, line)
	}

	return lines, nil
}

// NewHTTPClient creates a new HTTP client with
// insecure skip verify set to true and min TLS version set to 1.2.
func NewHTTPClient() *http.Client {
	// set InsecureSkipVerify to true to allow fetching
	// files form servers with self-signed certificates
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // skipcq: GSC-G402
			MinVersion:         tls.VersionTLS12,
		},
	}

	return &http.Client{Transport: tr}
}

// AdjustFileACLs takes the given fs path, tries to load
// the access file acl of that path and adds ACL rules
// rwx for the SUDO_UID and r-x for the SUDO_GID group.
func AdjustFileACLs(fsPath string) error {
	/// here we trust sudo to set up env variables
	// a missing SUDO_UID env var indicates the root user
	// runs clab without sudo
	uid, isSet := os.LookupEnv("SUDO_UID")
	if !isSet {
		// nothing to do, already running as root
		return nil
	}

	gid, isSet := os.LookupEnv("SUDO_GID")
	if !isSet {
		return fmt.Errorf("unable to retrieve GID. will only adjust UID for %q", fsPath)
	}

	iUID, err := strconv.Atoi(uid)
	if err != nil {
		return fmt.Errorf("unable to convert SUDO_UID %q to int", uid)
	}

	iGID, err := strconv.Atoi(gid)
	if err != nil {
		return fmt.Errorf("unable to convert SUDO_GID %q to int", gid)
	}

	// create a new ACL instance
	a := &acls.ACL{}
	// load the existing ACL entries of the PosixACLAccess type
	err = a.Load(fsPath, acls.PosixACLAccess)
	if err != nil {
		return err
	}

	// add an entry for the group
	err = a.AddEntry(acls.NewEntry(acls.TAG_ACL_GROUP, uint32(iGID), 5))
	if err != nil {
		return err
	}

	// add an entry for the User
	err = a.AddEntry(acls.NewEntry(acls.TAG_ACL_USER, uint32(iUID), 7))
	if err != nil {
		return err
	}

	// set the mask entry
	err = a.AddEntry(acls.NewEntry(acls.TAG_ACL_MASK, math.MaxUint32, 7))
	if err != nil {
		return err
	}

	// apply the ACL and return the error result
	err = a.Apply(fsPath, acls.PosixACLAccess)
	if err != nil {
		return err
	}

	return a.Apply(fsPath, acls.PosixACLDefault)
}

// SetUIDAndGID changes the UID and GID
// of the given path recursively to the values taken from
// SUDO_UID and SUDO_GID. Which should reflect be the non-root
// user that called clab via sudo.
func SetUIDAndGID(fsPath string) error {
	// here we trust sudo to set up env variables
	// a missing SUDO_UID env var indicates the root user
	// runs clab without sudo
	uid, isSet := os.LookupEnv("SUDO_UID")
	if !isSet {
		// nothing to do, already running as root
		return nil
	}

	gid, isSet := os.LookupEnv("SUDO_GID")
	if !isSet {
		return errors.New("failed to lookup SUDO_GID env var")
	}

	iUID, err := strconv.Atoi(uid)
	if err != nil {
		return fmt.Errorf("unable to convert SUDO_UID %q to int", uid)
	}

	iGID, err := strconv.Atoi(gid)
	if err != nil {
		return fmt.Errorf("unable to convert SUDO_GID %q to int", gid)
	}

	err = recursiveChown(fsPath, iUID, iGID)
	if err != nil {
		return err
	}

	return nil
}

// recursiveChown function recursively chowns a path.
func recursiveChown(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = os.Chown(name, uid, gid)
		}

		return err
	})
}
