package utils

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var RepoParserRegistry = NewRepoParserRegistry(
	&RepoParser{"github", ParseGitHubRepoUrl},
)

var errInvalidGithubURL = errors.New("invalid Github URL")

// GitHubGitRepo struct holds the parsed github url.
type GitHubGitRepo struct {
	URLBase        url.URL
	ProjectOwner   string
	RepositoryName string
	GitBranch      string
	Path           []string
	FileName       string
}

// ParseGitHubRepoUrl parses the github.com string url into the GithubURL struct.
func ParseGitHubRepoUrl(ghURL string) (GitRepo, error) {
	if !IsGitHubURL(ghURL) {
		return nil, fmt.Errorf("not a github url %q", ghURL)
	}

	u := &GitHubGitRepo{}

	// strip trailing slash
	ghURL = strings.TrimSuffix(ghURL, "/")

	parsedURL, err := url.Parse(ghURL)
	if err != nil {
		return nil, err
	}

	splitPath := strings.Split(strings.TrimPrefix(parsedURL.Path, "/"), "/")

	// path need to hold at least 2 elements,
	// user / org and repo
	if len(splitPath) < 2 || splitPath[0] == "" || splitPath[1] == "" {
		return nil, fmt.Errorf("%w %s", errInvalidGithubURL, ghURL)
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
	splitPath[1] = strings.TrimSuffix(splitPath[1], ".git")
	u.RepositoryName = splitPath[1]

	switch {
	case len(splitPath) == 2:
		return u, nil
	case len(splitPath) < 4:
		return nil, fmt.Errorf("%w invalid github path. should have either 2 or >= 4 path elements", errInvalidGithubURL)
	}

	u.GitBranch = splitPath[3]

	switch {
	// path points to a file at a specific git ref
	case splitPath[2] == "blob":
		if !(strings.HasSuffix(ghURL, ".yml") || strings.HasSuffix(ghURL, ".yaml")) {
			return nil, errInvalidGithubURL
		}
		u.Path = splitPath[4 : len(splitPath)-1]
		u.FileName = splitPath[len(splitPath)-1]
	// path points to a git ref (branch or tag)
	case splitPath[2] == "tree":
		if splitPath[len(splitPath)-1] == "" {
			return nil, errInvalidGithubURL
		}
		u.Path = splitPath[4:]
		u.FileName = "" // no filename, a dir is referenced
	}

	return u, nil
}

// GetFilename returns the filename if a file was specifically referenced.
// the empty string is returned otherwise.
func (u *GitHubGitRepo) GetFilename() string {
	return u.FileName
}

// Returns the path within the repository that was pointed to
func (u *GitHubGitRepo) GetPath() []string {
	return u.Path
}

// GetRepoName returns the repository name
func (u *GitHubGitRepo) GetRepoName() string {
	return u.RepositoryName
}

// GetBranch returns the referenced Git branch name.
// the empty string is returned otherwise.
func (u *GitHubGitRepo) GetBranch() string {
	if u.GitBranch == "" {
		return "HEAD"
	}
	return u.GitBranch
}

// GetRepoUrl returns the URL of the repository
func (u *GitHubGitRepo) GetRepoUrl() *url.URL {
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
	return nil, fmt.Errorf("%w unable to determine repo parser for %q", errInvalidGithubURL, url)
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
