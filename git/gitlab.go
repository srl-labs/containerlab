package git

import (
	"fmt"
	neturl "net/url"
	"strings"
)

// IsGitLabURL returns true if the given url is a gitlab url.
func IsGitLabURL(u *neturl.URL) bool {
	// besides simply looking for gitlab substring in the url,
	// we should consider fetching the http response and check
	// for gitlab string in the body.
	return strings.Contains(u.String(), "gitlab")
}

// GitLabRepo represents a gitlab repository.
type GitLabRepo struct {
	GitRepoStruct
}

// NewGitLabRepoFromURL parses the given url and returns a GitLabRepo.
func NewGitLabRepoFromURL(url *neturl.URL) (*GitLabRepo, error) {
	r := &GitLabRepo{
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

	// set CloneURL to the copy of the original URL
	// this is overridden later for repo urls with
	// paths containing blob or tree elements
	r.CloneURL = &neturl.URL{}
	*r.CloneURL = *r.URL
	// remove raw query from the clone url
	// raw query is ref_type=heads
	// in https://gitlab.com/rdodin/clab-test-repo/-/tree/branch1?ref_type=heads
	r.CloneURL.RawQuery = ""

	// remove trailing slash from the path
	// as it bears no meaning for the clone url
	r.CloneURL.Path = strings.TrimSuffix(r.CloneURL.Path, "/")

	r.ProjectOwner = splitPath[0]

	// in case repo url has a trailing .git suffix, trim it
	r.RepositoryName = strings.TrimSuffix(splitPath[1], ".git")

	switch {
	case len(splitPath) == 2:
		return r, nil
	case len(splitPath) < 5:
		return nil, fmt.Errorf("%w invalid github path. should have either 2 or >= 5 path elements", errInvalidURL)
	}

	r.GitBranch = splitPath[4]

	switch splitPath[3] {
	// path points to a file at a specific git ref
	case "blob":
		if !strings.HasSuffix(r.URL.Path, ".yml") &&
			!strings.HasSuffix(r.URL.Path, ".yaml") {
			return nil, fmt.Errorf("%w: topology file must have yml or yaml extension", errInvalidURL)
		}

		if len(splitPath)-1 > 5 {
			r.Path = splitPath[5 : len(splitPath)-1]
		}

		// overriding CloneURL Path to point to the git repo
		r.CloneURL.Path = "/" + splitPath[0] + "/" + splitPath[1]

		r.FileName = splitPath[len(splitPath)-1]

	// path points to a git ref (branch or tag)
	case "tree":
		if len(splitPath) > 5 {
			r.Path = splitPath[5:]
		}

		// overriding CloneURL Path to point to the git repo
		r.CloneURL.Path = "/" + splitPath[0] + "/" + splitPath[1]

		r.FileName = "" // no filename, a dir is referenced
	}

	return r, nil
}
