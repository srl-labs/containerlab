// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"bufio"
	"context"
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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/steiler/acls"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
)

var (
	errNonRegularFile = errors.New("non-regular file")
	errHTTPFetch      = errors.New("failed to fetch http(s) resource")
	errS3Fetch        = errors.New("failed to fetch s3 resource")
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
func CopyFile(ctx context.Context, src, dst string, mode os.FileMode) (err error) {
	var sfi os.FileInfo
	if !IsHttpURL(src, false) && !IsS3URL(src) {
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

	err = os.MkdirAll(filepath.Dir(dst), 0o750)
	if err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	// Change file ownership to user running Containerlab instead of effective UID
	err = SetUIDAndGID(dst)
	if err != nil {
		return err
	}

	err = out.Chmod(mode)
	if err != nil {
		return err
	}

	defer func() {
		// should only err on repeated calls to close anyway
		_ = out.Close()
	}()

	return CopyFileContents(ctx, src, out)
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

	// if schemaless is not allowed and the string does not contain a schema, it is not an URL
	if !allowSchemaless && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}

	// if schemaless is allowed and the string does not contain a schema, but contains a dot
	// in any a non-domain portion then it is not a valid URL
	if allowSchemaless && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		split := strings.SplitN(s, "/", 2)
		if len(split) > 1 {
			if strings.Contains(split[1], ".") {
				return false
			}
		}
	}

	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}

	u, err := url.ParseRequestURI(s)

	return err == nil && u.Host != ""
}

// IsS3URL checks if the URL is an S3 URL (s3://bucket/key format).
func IsS3URL(s string) bool {
	return strings.HasPrefix(s, "s3://")
}

// ParseS3URL parses an S3 URL and returns the bucket and key.
func ParseS3URL(s3URL string) (bucket, key string, err error) {
	if !IsS3URL(s3URL) {
		return "", "", fmt.Errorf("not an S3 URL: %s", s3URL)
	}

	u, err := url.Parse(s3URL)
	if err != nil {
		return "", "", err
	}

	bucket = u.Host
	key = strings.TrimPrefix(u.Path, "/")

	if bucket == "" || key == "" {
		return "", "", fmt.Errorf("invalid S3 URL format: %s", s3URL)
	}

	return bucket, key, nil
}

func copyFileContentsS3(src string) (io.ReadCloser, error) {
	bucket, key, err := ParseS3URL(src)
	if err != nil {
		return nil, err
	}

	// Get region from environment, default to us-east-1
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	// Create credential chain that mimics AWS SDK behavior
	credProviders := []credentials.Provider{
		&credentials.EnvAWS{},                                             // 1. Environment variables
		&credentials.FileAWSCredentials{},                                 // 2. ~/.aws/credentials (default profile)
		&credentials.IAM{Client: &http.Client{Timeout: 10 * time.Second}}, // 3. IAM role (EC2/ECS/Lambda)
	}

	// Create MinIO client with chained credentials
	client, err := minio.New("s3.amazonaws.com", &minio.Options{
		Creds:  credentials.NewChainCredentials(credProviders),
		Secure: true,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Get object from S3
	object, err := client.GetObject(context.TODO(), bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", errS3Fetch, src, err)
	}

	// Verify object exists by reading its stats
	_, err = object.Stat()
	if err != nil {
		object.Close()
		return nil, fmt.Errorf("%w: %s: object not found or access denied: %v", errS3Fetch, src, err)
	}

	return object, nil
}

// CopyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
// src can be an http(s) URL or an S3 URL.
func CopyFileContents(ctx context.Context, src string, dst *os.File) (err error) {
	var in io.ReadCloser

	switch {
	case IsHttpURL(src, false):
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		}

		// download using client
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, http.NoBody)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			return fmt.Errorf("%w: %s", errHTTPFetch, src)
		}

		defer resp.Body.Close()

		in = resp.Body

	case IsS3URL(src):
		in, err = copyFileContentsS3(src)
		if err != nil {
			return err
		}
	default:
		in, err = os.Open(src)
		if err != nil {
			return err
		}
	}
	defer in.Close() // skipcq: GO-S2307

	_, err = io.Copy(dst, in)
	if err != nil {
		return err
	}

	return dst.Sync()
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

	// Change file ownership to user running Containerlab instead of effective UID
	err = SetUIDAndGID(file)
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

	switch p[0] {
	// resolve ~/ path
	case '~':
		p = ExpandHome(p)
	case '/':
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
func FilenameForURL(ctx context.Context, rawUrl string) string {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return UndefinedFileName
	}

	// try extracting the filename from "content-disposition" header
	if IsHttpURL(rawUrl, false) {
		client := NewHTTPClient()

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawUrl, http.NoBody)
		if err != nil {
			return filepath.Base(u.Path)
		}

		resp, err := client.Do(req)
		if err != nil {
			return filepath.Base(u.Path)
		}

		defer resp.Body.Close()

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

func GetRealUserIDs() (userUID, userGID int, err error) {
	// Here we check whether SUDO set the SUDO_UID and SUDO_GID variables
	sudoUID, isSudoUIDSet := os.LookupEnv("SUDO_UID")
	if isSudoUIDSet {
		userUID, err = strconv.Atoi(sudoUID)
		if err != nil {
			return -1, -1, fmt.Errorf("unable to convert SUDO_UID %q to int", sudoUID)
		}
		sudoGID, isSudoGIDSet := os.LookupEnv("SUDO_GID")
		if isSudoGIDSet {
			userGID, err = strconv.Atoi(sudoGID)
			if err != nil {
				return -1, -1, fmt.Errorf("unable to convert SUDO_GID %q to int", sudoGID)
			}
		}
		// Otherwise just check for the real UID/GID (instead of the effective UID)
	} else {
		userUID = os.Getuid()
		userGID = os.Getgid()
	}

	return userUID, userGID, nil
}

// AdjustFileACLs takes the given fs path, tries to load the access file acl of that path and adds ACL rules:
// rwx for the real UID user and r-x for the real GID group.
func AdjustFileACLs(fsPath string) error {
	userUID, userGID, err := GetRealUserIDs()
	if err != nil {
		return fmt.Errorf("unable to retrieve real user UID and GID: %v", err)
	}

	if userUID == 0 && userGID == 0 {
		// We are running as root without sudo, return early
		return nil
	}

	// create a new ACL instance
	a := &acls.ACL{}
	// load the existing ACL entries of the PosixACLAccess type
	err = a.Load(fsPath, acls.PosixACLAccess)
	if err != nil {
		return err
	}

	// add an entry for the group
	err = a.AddEntry(acls.NewEntry(acls.TAG_ACL_GROUP, uint32(userGID), 5))
	if err != nil {
		return err
	}

	// add an entry for the User
	err = a.AddEntry(acls.NewEntry(acls.TAG_ACL_USER, uint32(userUID), 7))
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

// SetUIDAndGID changes the UID and GID of the given path recursively to the values taken from getRealUserIDs,
// which should reflect the non-root user's UID and GID.
func SetUIDAndGID(fsPath string) error {
	userUID, userGID, err := GetRealUserIDs()
	if err != nil {
		return fmt.Errorf("unable to retrieve real user UID and GID: %v", err)
	}

	if userUID == 0 && userGID == 0 {
		// We are running as root without sudo, return early
		return nil
	}

	err = recursiveChown(fsPath, userUID, userGID)
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

var osRelease string

// GetOSRelease returns the OS release of the host by inspecting /etc/*-release files.
func GetOSRelease() string {
	// return cached result
	if osRelease != "" {
		return osRelease
	}
	osRelease = clabconstants.NotApplicable

	matches, err := filepath.Glob("/etc/*-release")
	if err != nil {
		return osRelease
	}

	re := regexp.MustCompile(`(DISTRIB_DESCRIPTION|PRETTY_NAME)="(.*)"`)

	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			log.Error(err)
		}

		match := re.FindSubmatch(data)
		// [0] = whole line match, [1] = left side of "=", [2] = right side of "="
		if len(match) >= 3 {
			osRelease = string(match[2])
			break
		}
	}

	return osRelease
}
