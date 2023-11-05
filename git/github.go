package git

import (
	"fmt"
	"net/url"
	neturl "net/url"
	"strings"
)

// NewGitRepoFromURL parses the given url and returns a GitHubRepo.
func NewGitHubRepoFromURL(url *neturl.URL) (*GitHubRepo, error) {
	r := &GitHubRepo{
		GitRepoStruct: GitRepoStruct{
			URL: url,
		}}

	// trimming the leading and trailing slashes
	// so that splitPath will have the slashes between the elements only
	splitPath := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	// path needs to hold at least 2 elements,
	// user / org and repo
	if len(splitPath) < 2 || splitPath[0] == "" || splitPath[1] == "" {
		return nil, fmt.Errorf("%w %s", errInvalidURL, r.URL.String())
	}

	// github.dev links can be cloned using github.com
	if r.URL.Host == "github.dev" {
		r.URL.Host = "github.com"
	}

	// set CloneURL to the copy of the original URL
	r.CloneURL = &neturl.URL{}
	*r.CloneURL = *r.URL

	// remove trailing slash from the path
	// as it bears no meaning for the clone url
	r.CloneURL.Path = strings.TrimSuffix(r.CloneURL.Path, "/")

	r.ProjectOwner = splitPath[0]

	// in case repo url has a trailing .git suffix, trim it
	r.RepositoryName = strings.TrimSuffix(splitPath[1], ".git")

	switch {
	case len(splitPath) == 2:
		return r, nil
	case len(splitPath) < 4:
		return nil, fmt.Errorf("%w invalid github path. should have either 2 or >= 4 path elements", errInvalidURL)
	}

	r.GitBranch = splitPath[3]

	switch {
	// path points to a file at a specific git ref
	case splitPath[2] == "blob":
		if !(strings.HasSuffix(r.URL.Path, ".yml") || strings.HasSuffix(r.URL.Path, ".yaml")) {
			return nil, errInvalidURL
		}

		if len(splitPath)-1 > 4 {
			r.Path = splitPath[4 : len(splitPath)-1]
		}

		r.FileName = splitPath[len(splitPath)-1]

	// path points to a git ref (branch or tag)
	case splitPath[2] == "tree":
		if splitPath[len(splitPath)-1] == "" {
			return nil, errInvalidURL
		}
		r.Path = splitPath[4:]
		r.FileName = "" // no filename, a dir is referenced
	}

	return r, nil
}

// IsGitHubURL checks if the url is a github url.
func IsGitHubURL(url *url.URL) bool {
	return strings.Contains(url.Host, "github.com") ||
		strings.Contains(url.Host, "github.dev")
}

// GitHubRepo struct holds the parsed github url.
type GitHubRepo struct {
	GitRepoStruct
}
