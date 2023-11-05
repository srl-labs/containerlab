package git

import (
	"fmt"
	"net/url"
	"strings"
)

func IsGitLabURL(u *url.URL) bool {
	// we're looking for the "gitlab" sub-string in the entire URL
	// we probably need a better strategy here. Anyways it is working for now.
	return strings.Contains(u.String(), "gitlab")
}

type GitLabRepo struct {
	GitRepoStruct
}

func (r *GitLabRepo) ParseURL() error {
	splitPath := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	// path need to hold at least 2 elements,
	// user / org and repo
	if len(splitPath) < 2 || splitPath[0] == "" || splitPath[1] == "" {
		return fmt.Errorf("%w %s", errInvalidURL, r.URL.String())
	}

	r.URL.Fragment = "" // reset fragment
	r.URL.Path = ""     // reset path
	r.URL.RawQuery = "" // reset rawquery

	r.ProjectOwner = splitPath[0]

	// in case repo url has a trailing .git suffix, trim it
	r.RepositoryName = strings.TrimSuffix(splitPath[1], ".git")

	switch {
	case len(splitPath) == 2:
		return nil
	case len(splitPath) < 5:
		return fmt.Errorf("%w invalid github path. should have either 2 or >= 5 path elements", errInvalidURL)
	}

	r.GitBranch = splitPath[4]

	switch {
	// path points to a file at a specific git ref
	case splitPath[3] == "blob":
		if !(strings.HasSuffix(r.URL.Path, ".yml") || strings.HasSuffix(r.URL.Path, ".yaml")) {
			return fmt.Errorf("%w referenced file must be *.yml or *.yaml. %q is therefor invalid", errInvalidURL, splitPath[len(splitPath)-1])
		}
		r.Path = splitPath[5 : len(splitPath)-1]
		r.FileName = splitPath[len(splitPath)-1]
	// path points to a git ref (branch or tag)
	case splitPath[3] == "tree":
		if splitPath[len(splitPath)-1] == "" {
			return errInvalidURL
		}
		r.Path = splitPath[5:]
		r.FileName = "" // no filename, a dir is referenced
	}

	return nil
}
