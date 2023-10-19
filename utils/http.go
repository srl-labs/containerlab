package utils

import (
	"errors"
	"net/url"
	"os/exec"
	"strings"
	"os"
)

// GithubURI struct holds the parsed github url
type GithubURI struct {
	URLBase string
	projectOwner string
	RepositoryName string
	gitBranch string
	FileName string
}

// NewGithubURI returns a pointer to a GithubURI struct
func NewGithubURI() *GithubURI { 
	return &GithubURI{}
}

// TokenizeGithubURL parses the github url and updates a GithubURI struct
func TokenizeGithubURL(uri string, githubURIStruct *GithubURI) error {
	suffix, err := HasSupportedSuffix(uri)
	if err != nil {
		return err
	}
	uriParsed, err := url.Parse(uri)
	if err != nil {
		return err
	}
	// split the url path and remove the first empty element
	splitUrl := strings.Split(uriParsed.Path, "/")[1:]
	githubURIStruct.URLBase = uriParsed.Scheme + "://" + uriParsed.Host
	githubURIStruct.projectOwner = splitUrl[0]
	githubURIStruct.RepositoryName = splitUrl[1]
	if strings.Contains(uri, "raw.githubusercontent.com") && suffix == ".yml" || suffix == ".yaml" {
		githubURIStruct.URLBase = "https://github.com"
		githubURIStruct.gitBranch = splitUrl[2]
		githubURIStruct.FileName = splitUrl[len(splitUrl)-1]
	} else if strings.Contains(uri, "github.com")  && suffix == ".yml" || suffix == ".yaml" {
		githubURIStruct.gitBranch = splitUrl[3]
		githubURIStruct.FileName = splitUrl[len(splitUrl)-1]
	} else if strings.Contains(uri, "github.com") && suffix == ".git" || suffix == ""{
		// if lenth of the slice of url path is greater than 3, it means that the user has passed in a repo with a branch
		if len(splitUrl) > 3 && splitUrl[2] == "tree"{
			githubURIStruct.gitBranch = splitUrl[3]
		// if the length equals 2 they have passed in a repo without a branch
		} else if len(splitUrl) == 2 {
			updatedRepoName := strings.Split(splitUrl[1], ".git")[0]
			githubURIStruct.RepositoryName = updatedRepoName
			githubURIStruct.gitBranch = "main"
		} else {
			return errors.New("unsupported git repositoy URI format")
		}
	}
	return nil
}

// RetrieveGithubRepo clones the github repo into the current directory
func RetrieveGithubRepo(githubURIStruct *GithubURI) (error) {
	cmd := exec.Command("git", "clone", githubURIStruct.URLBase + "/" + githubURIStruct.projectOwner + "/" + githubURIStruct.RepositoryName + ".git", "--branch", githubURIStruct.gitBranch, "--depth", "1" )
	cmd.Dir = "./"
	err := cmd.Run()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err != nil {
		return err
	}
	return nil
}

// IsGitHubURL checks if the url is a github url
func IsGitHubURL(url string) bool {
	return strings.Contains(url, "github")
}

// ErrInvalidSuffix is returned when the url passed in does not have a supported suffix, global function was required for test cases to work
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
