package utils

import (
	"errors"
	"net/url"
	"os/exec"
	"strings"
	"os"
)
type GithubURIType struct {
	RawWithYaml bool
	BaseWithYaml bool
	BaseWithGit bool
}

type GithubURI struct {
	urlBase string
	projectOwner string
	RepositoryName string
	gitBranch string
	FileName string
	uriType GithubURIType
}

func NewGithubURI() *GithubURI { 
	return &GithubURI{}
}


func TokenizeGithubURL(uri string, githubURIStruct *GithubURI) error {
	suffix, err := HasSupportedSuffix(uri)
	if err != nil {
		return err
	}
	uriParsed, err := url.Parse(uri)
	if err != nil {
		return err
	}
	splitUrl := strings.Split(uriParsed.Path, "/")[1:]
	// copy(splitUrl[1:], splitUrl)
	githubURIStruct.urlBase = uriParsed.Scheme + "://" + uriParsed.Host
	githubURIStruct.projectOwner = splitUrl[0]
	githubURIStruct.RepositoryName = splitUrl[1]
	if strings.Contains(uri, "raw.githubusercontent.com") && suffix == ".yml" || suffix == ".yaml" {
		githubURIStruct.urlBase = "https://github.com"
		githubURIStruct.gitBranch = splitUrl[2]
		githubURIStruct.FileName = splitUrl[len(splitUrl)-1]
		githubURIStruct.uriType.RawWithYaml = true
	} else if strings.Contains(uri, "github.com")  && suffix == ".yml" || suffix == ".yaml" {
		githubURIStruct.gitBranch = splitUrl[3]
		githubURIStruct.FileName = splitUrl[len(splitUrl)-1]
		githubURIStruct.uriType.BaseWithYaml = true
	} else if strings.Contains(uri, "github.com") && suffix == ".git" || suffix == ""{
		if len(splitUrl) > 3 && splitUrl[2] == "tree"{
			githubURIStruct.gitBranch = splitUrl[3]
		} else if len(splitUrl) == 2 {
			// updatedRepoName := splitUrl[1]
			updatedRepoName := strings.Split(splitUrl[1], ".git")[0]
			githubURIStruct.RepositoryName = updatedRepoName
			githubURIStruct.gitBranch = "main"
		} else {
			return errors.New("unsupported git repositoy URI format")
		}
		githubURIStruct.uriType.BaseWithGit = true
	}
	return nil
}

func RetrieveGithubRepo(githubURIStruct *GithubURI) (error) {
	cmd := exec.Command("git", "clone", githubURIStruct.urlBase + "/" + githubURIStruct.projectOwner + "/" + githubURIStruct.repositoyName + ".git", "--branch", githubURIStruct.gitBranch, "--depth", "1" )
	cmd.Dir = "./"
	err := cmd.Run()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err != nil {
		return err
	}
	return nil
}


func IsGitHubURL(url string) bool {
	return strings.Contains(url, "github")

}

// required global variable for tests, otherwise comparison operator fails as error instances were not equal
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
