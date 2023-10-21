package utils

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

var errInvalidGithubURL = errors.New("invalid Github URL")

// GithubURL struct holds the parsed github url.
type GithubURL struct {
	URLBase        string
	ProjectOwner   string
	RepositoryName string
	GitBranch      string
	FileName       string
}

// NewGithubURL returns a pointer to a GithubURI struct
func NewGithubURL() *GithubURL {
	return &GithubURL{}
}

// Parse parses the github.com string url into the GithubURL struct.
func (u *GithubURL) Parse(ghURL string) error {
	parsedURL, err := url.Parse(ghURL)
	if err != nil {
		return err
	}

	splitPath := strings.Split(parsedURL.Path, "/")

	if len(splitPath) < 3 || splitPath[2] == "" {
		return fmt.Errorf("%w %s", errInvalidGithubURL, ghURL)
	}

	u.URLBase = parsedURL.Scheme + "://" + parsedURL.Host
	u.ProjectOwner = splitPath[1]

	// in case repo url has a trailing .git suffix, trim it
	splitPath[2] = strings.TrimSuffix(splitPath[2], ".git")
	u.RepositoryName = splitPath[2]

	switch {
	// path points to a file at a specific git ref
	case strings.Contains(ghURL, "/blob/"):
		if splitPath[len(splitPath)-1] == "" || !(strings.HasSuffix(ghURL, ".yml") || strings.HasSuffix(ghURL, ".yaml")) {
			return errInvalidGithubURL
		}

		u.FileName = splitPath[len(splitPath)-1]
		u.GitBranch = splitPath[len(splitPath)-2]

	// path points to a git ref (branch or tag)
	case strings.Contains(ghURL, "/tree/"):
		if splitPath[len(splitPath)-1] == "" {
			return errInvalidGithubURL
		}

		u.GitBranch = splitPath[len(splitPath)-1]
	}

	return nil
}

// RetrieveGithubRepo clones the github repo into the current directory.
func RetrieveGithubRepo(githubURL *GithubURL) error {
	cmd := exec.Command("git", "clone", githubURL.URLBase+"/"+githubURL.ProjectOwner+"/"+githubURL.RepositoryName+".git", "--branch", githubURL.GitBranch, "--depth", "1")
	cmd.Dir = "./"
	err := cmd.Run()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err != nil {
		return err
	}

	return nil
}

// IsGitHubURL checks if the url is a github url.
func IsGitHubURL(url string) bool {
	return strings.Contains(url, "github.com") || strings.Contains(url, "raw.githubusercontent.com")
}
