package git

import (
	"errors"
	"net/url"
)

var errInvalidURL = errors.New("invalid URL")

// GitRepoStruct is a struct that contains all the fields
// required for a GitRepo instance.
type GitRepoStruct struct {
	URL            *url.URL
	ProjectOwner   string
	RepositoryName string
	GitBranch      string
	Path           []string
	FileName       string
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
	return u.URL.JoinPath(u.ProjectOwner, u.RepositoryName)
}

type GitRepo interface {
	GetRepoName() string
	GetFilename() string
	GetPath() []string
	GetRepoUrl() *url.URL
	GetBranch() string
	// ParseGitURL parses the git url into GitRepo struct.
	ParseGitURL() error
}

// NewGitRepo parses the given git urlPath and returns an interface
// that is backed by Github or Gitlab repo implementations.
func NewGitRepo(urlPath string) (GitRepo, error) {
	var r GitRepo
	var err error

	u, err := url.Parse(urlPath)
	if err != nil {
		return nil, err
	}

	if IsGitHubURL(u) {
		r = &GitHubRepo{
			GitRepoStruct{
				URL: u,
			}}
	}

	if IsGitLabURL(u) {
		r = &GitLabRepo{
			GitRepoStruct{
				URL: u,
			}}
	}

	err = r.ParseGitURL()

	return r, err
}
