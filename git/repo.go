package git

import (
	"errors"
	"net/url"
)

var errInvalidURL = errors.New("invalid URL")

// GitRepoStruct is a struct that contains all the fields
// required for a GitRepo instance.
type GitRepoStruct struct {
	// original URL parsed from the user input
	URL *url.URL
	// CloneURL is the URL that will be used for cloning the repo
	CloneURL       *url.URL
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

// GetName returns the repository name
func (u *GitRepoStruct) GetName() string {
	return u.RepositoryName
}

// GetBranch returns the referenced Git branch name.
// the empty string is returned otherwise.
func (u *GitRepoStruct) GetBranch() string {
	return u.GitBranch
}

// GetCloneURL returns the CloneURL of the repository
func (u *GitRepoStruct) GetCloneURL() *url.URL {
	return u.CloneURL
}

type GitRepo interface {
	GetName() string
	GetFilename() string
	GetPath() []string
	GetCloneURL() *url.URL
	GetBranch() string
	// ParseURL parses the user-provided url into GitRepo struct.
	ParseURL() error
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

	err = r.ParseURL()

	return r, err
}
