package utils

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var RepoParserRegistry = NewRepoParserRegistry(
	&RepoParser{"github", ParseGitHubRepoUrl},
	&RepoParser{"gitlab", ParseGitLabRepoUrl},
)

var errInvalidURL = errors.New("invalid URL")

// GitRepoStruct is a struct that contains all the fields
// required for a GitRepo instance.
type GitRepoStruct struct {
	URLBase        url.URL
	ProjectOwner   string
	RepositoryName string
	GitBranch      string
	Path           []string
	FileName       string
}

// GitHubGitRepo struct holds the parsed github url.
type GitHubGitRepo struct {
	GitRepoStruct
}

type GitLabGitRepo struct {
	GitRepoStruct
}

func ParseGitLabRepoUrl(urlStr string) (GitRepo, error) {

	if !IsGitLabURL(urlStr) {
		return nil, fmt.Errorf("not a gitlab url %q", urlStr)
	}

	u := &GitLabGitRepo{}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	splitPath := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")

	// path need to hold at least 2 elements,
	// user / org and repo
	if len(splitPath) < 2 || splitPath[0] == "" || splitPath[1] == "" {
		return nil, fmt.Errorf("%w %s", errInvalidURL, urlStr)
	}

	u.URLBase = *parsedURL  // copy parsed url
	u.URLBase.Fragment = "" // reset fragment
	u.URLBase.Path = ""     // reset path
	u.URLBase.RawQuery = "" // reset rawquery

	u.ProjectOwner = splitPath[0]

	// in case repo url has a trailing .git suffix, trim it
	u.RepositoryName = strings.TrimSuffix(splitPath[1], ".git")

	switch {
	case len(splitPath) == 2:
		return u, nil
	case len(splitPath) < 5:
		return nil, fmt.Errorf("%w invalid github path. should have either 2 or >= 5 path elements", errInvalidURL)
	}

	u.GitBranch = splitPath[4]

	switch {
	// path points to a file at a specific git ref
	case splitPath[3] == "blob":
		if !(strings.HasSuffix(parsedURL.Path, ".yml") || strings.HasSuffix(parsedURL.Path, ".yaml")) {
			return nil, fmt.Errorf("referenced file must be *.yml or *.yaml. %q is therefor invlaid", splitPath[len(splitPath)-1])
		}
		u.Path = splitPath[5 : len(splitPath)-1]
		u.FileName = splitPath[len(splitPath)-1]
	// path points to a git ref (branch or tag)
	case splitPath[3] == "tree":
		if splitPath[len(splitPath)-1] == "" {
			return nil, errInvalidURL
		}
		u.Path = splitPath[5:]
		u.FileName = "" // no filename, a dir is referenced
	}

	return u, nil
}

func IsGitLabURL(url string) bool {
	return strings.Contains(url, "gitlab")
}

// ParseGitHubRepoUrl parses the github.com string url into the GithubURL struct.
func ParseGitHubRepoUrl(ghURL string) (GitRepo, error) {
	if !IsGitHubURL(ghURL) {
		return nil, fmt.Errorf("not a github url %q", ghURL)
	}

	u := &GitHubGitRepo{}

	parsedURL, err := url.Parse(ghURL)
	if err != nil {
		return nil, err
	}

	splitPath := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")

	// path need to hold at least 2 elements,
	// user / org and repo
	if len(splitPath) < 2 || splitPath[0] == "" || splitPath[1] == "" {
		return nil, fmt.Errorf("%w %s", errInvalidURL, ghURL)
	}

	// github.dev links can be cloned using github.com
	if parsedURL.Host == "github.dev" {
		parsedURL.Host = "github.com"
	}

	u.URLBase = *parsedURL  // copy parsed url
	u.URLBase.Fragment = "" // reset fragment
	u.URLBase.Path = ""     // reset path
	u.URLBase.RawQuery = "" // reset rawquery

	u.ProjectOwner = splitPath[0]

	// in case repo url has a trailing .git suffix, trim it
	u.RepositoryName = strings.TrimSuffix(splitPath[1], ".git")

	switch {
	case len(splitPath) == 2:
		return u, nil
	case len(splitPath) < 4:
		return nil, fmt.Errorf("%w invalid github path. should have either 2 or >= 4 path elements", errInvalidURL)
	}

	u.GitBranch = splitPath[3]

	switch {
	// path points to a file at a specific git ref
	case splitPath[2] == "blob":
		if !(strings.HasSuffix(parsedURL.Path, ".yml") || strings.HasSuffix(parsedURL.Path, ".yaml")) {
			return nil, errInvalidURL
		}
		u.Path = splitPath[4 : len(splitPath)-1]
		u.FileName = splitPath[len(splitPath)-1]
	// path points to a git ref (branch or tag)
	case splitPath[2] == "tree":
		if splitPath[len(splitPath)-1] == "" {
			return nil, errInvalidURL
		}
		u.Path = splitPath[4:]
		u.FileName = "" // no filename, a dir is referenced
	}

	return u, nil
}

// GetFilename returns the filename if a file was specifically referenced.
// the empty string is returned otherwise.
func (u *GitRepoStruct) GetFilename() string {
	return u.FileName
}

// Returns the path within the repository that was pointed to
func (u *GitRepoStruct) GetPath() []string {
	return u.Path
}

// GetRepoName returns the repository name
func (u *GitRepoStruct) GetRepoName() string {
	return u.RepositoryName
}

// GetBranch returns the referenced Git branch name.
// the empty string is returned otherwise.
func (u *GitRepoStruct) GetBranch() string {
	return u.GitBranch
}

// GetRepoUrl returns the URL of the repository
func (u *GitRepoStruct) GetRepoUrl() *url.URL {
	return u.URLBase.JoinPath(u.ProjectOwner, u.RepositoryName)
}

// IsGitHubURL checks if the url is a github url.
func IsGitHubURL(url string) bool {
	return strings.Contains(url, "github.com") ||
		strings.Contains(url, "github.dev")
}

type GitRepo interface {
	GetRepoName() string
	GetFilename() string
	GetPath() []string
	GetRepoUrl() *url.URL
	GetBranch() string
}

type RepositoryParserRegistry struct {
	Parser []*RepoParser
}

func (r *RepositoryParserRegistry) Parse(url string) (GitRepo, error) {
	var err error
	var repo GitRepo
	for _, p := range r.Parser {
		repo, err = p.Parser(url)
		if err == nil {
			return repo, nil
		}
	}
	return nil, fmt.Errorf("%w unable to determine repo parser for %q", errInvalidURL, url)
}

func NewRepoParserRegistry(rps ...*RepoParser) *RepositoryParserRegistry {
	reg := &RepositoryParserRegistry{}
	for _, rp := range rps {
		reg.AddRepoParser(rp)
	}
	return reg
}

func (r *RepositoryParserRegistry) AddRepoParser(rp *RepoParser) {
	r.Parser = append(r.Parser, rp)
}

type RepoParser struct {
	Name   string
	Parser ParserFunc
}

type ParserFunc func(string) (GitRepo, error)
