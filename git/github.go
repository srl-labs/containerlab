package git

import (
	"fmt"
	"strings"

	neturl "net/url"
)

// NewGitHubRepoFromURL parses the given url and returns a GitHubRepo.
func NewGitHubRepoFromURL(url *neturl.URL) (*GitHubRepo, error) {
	r := &GitHubRepo{
		GitRepoStruct: GitRepoStruct{
			URL: url,
		},
	}

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
	// this is overridden later for repo urls with
	// paths containing blob or tree elements
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

	switch splitPath[2] {
	// path points to a file at a specific git ref
	case "blob":
		if !strings.HasSuffix(r.URL.Path, ".yml") &&
			!strings.HasSuffix(r.URL.Path, ".yaml") {
			return nil, fmt.Errorf("%w: topology file must have yml or yaml extension", errInvalidURL)
		}

		if len(splitPath)-1 > 4 {
			r.Path = splitPath[4 : len(splitPath)-1]
		}

		// overriding CloneURL Path to point to the git repo
		r.CloneURL.Path = "/" + splitPath[0] + "/" + splitPath[1]

		r.FileName = splitPath[len(splitPath)-1]

	// path points to a git ref (branch or tag)
	case "tree":
		if len(splitPath) > 4 {
			r.Path = splitPath[4:]
		}

		// overriding CloneURL Path to point to the git repo
		r.CloneURL.Path = "/" + splitPath[0] + "/" + splitPath[1]

		r.FileName = "" // no filename, a dir is referenced
	}

	return r, nil
}

// IsGitHubURL checks if the url is a github url.
func IsGitHubURL(url *neturl.URL) bool {
	return strings.Contains(url.Host, "github.com") ||
		strings.Contains(url.Host, "github.dev")
}

// GitHubRepo struct holds the parsed github url.
type GitHubRepo struct {
	GitRepoStruct
}

// IsGitHubShortURL returns true for github-friendly short urls
// such as srl-labs/containerlab.
func IsGitHubShortURL(s string) bool {
	split := strings.Split(s, "/")
	// only 2 elements are allowed
	if len(split) != 2 {
		return false
	}

	// dot is not allowed in the project owner
	if strings.Contains(split[0], ".") {
		return false
	}

	return true
}
