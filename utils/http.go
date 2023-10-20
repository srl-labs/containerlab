package utils

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// GithubURL struct holds the parsed github url.
type GithubURL struct {
	URLBase        string
	projectOwner   string
	RepositoryName string
	gitBranch      string
	FileName       string
}

// NewGithubURL returns a pointer to a GithubURI struct
func NewGithubURL() *GithubURL {
	return &GithubURL{}
}

// Tokenize parses the string url.
func (u *GithubURL) Tokenize(ghURL string) error {
	suffix, err := HasSupportedSuffix(ghURL)
	if err != nil {
		return err
	}

	parsedURL, err := url.Parse(ghURL)
	if err != nil {
		return err
	}

	// split the url path and remove the first empty element
	splitUrl := strings.Split(parsedURL.Path, "/")[1:]
	u.URLBase = parsedURL.Scheme + "://" + parsedURL.Host
	u.projectOwner = splitUrl[0]
	u.RepositoryName = splitUrl[1]

	switch {
	case strings.Contains(ghURL, "raw.githubusercontent.com") && (suffix == ".yml" || suffix == ".yaml"):
		u.URLBase = "https://github.com"
		u.gitBranch = splitUrl[2]
		u.FileName = splitUrl[len(splitUrl)-1]
	case strings.Contains(ghURL, "github.com") && suffix == ".yml" || suffix == ".yaml":
		u.gitBranch = splitUrl[3]
		u.FileName = splitUrl[len(splitUrl)-1]
	case strings.Contains(ghURL, "github.com") && suffix == ".git" || suffix == "":
		// if lenth of the slice of url path is greater than 3, it means that the user has passed in a repo with a branch
		if len(splitUrl) > 3 && splitUrl[2] == "tree" {
			u.gitBranch = splitUrl[3]
			// if the length equals 2 they have passed in a repo without a branch
		} else if len(splitUrl) == 2 {
			updatedRepoName := strings.Split(splitUrl[1], ".git")[0]
			u.RepositoryName = updatedRepoName
			u.gitBranch = "main"
		} else {
			return errors.New("unsupported git repositoy URI format")
		}
	}

	return nil
}

// RetrieveGithubRepo clones the github repo into the current directory.
func RetrieveGithubRepo(githubURL *GithubURL) error {
	cmd := exec.Command("git", "clone", githubURL.URLBase+"/"+githubURL.projectOwner+"/"+githubURL.RepositoryName+".git", "--branch", githubURL.gitBranch, "--depth", "1")
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

// ErrInvalidSuffix is returned when the url passed in does not have a supported suffix, global function was required for test cases to work.
var ErrInvalidSuffix = errors.New("invalid uri path passed as topology argument, supported suffixes are .yml, .yaml, .git, or no suffix at all")

func HasSupportedSuffix(url string) (string, error) {
	// ckecks if the url has a valid suffix, if not it returns an error
	supported_suffix := []string{".yml", ".yaml", ".git", ""}
	for _, suffix := range supported_suffix {
		if strings.HasSuffix(url, suffix) {
			return suffix, nil
		}
	}

	return "", ErrInvalidSuffix
}
