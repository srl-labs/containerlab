package docker

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
)

const (
	dockerDefaultConfigDir  = ".docker"
	dockerDefaultConfigFile = "config.json"
)

type DockerConfigAuth struct {
	Auth string
}

// DockerConfig represents the docker config that is typically contained within ~/.docker/config.json.
type DockerConfig struct {
	Auths map[string]DockerConfigAuth `json:"auths,omitempty"`
}

func getImageDomainName(imageName string) string {
	var imageDomainName string

	imageRef, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		imageDomainName = ""
		log.Errorf("Unable to fetch image normalized name, error: %v", err)
	} else {
		imageDomainName = reference.Domain(imageRef)
	}

	return imageDomainName
}

func getDockerConfigPath(configPath string) (string, error) {
	var err error
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		configPath = filepath.Join(homeDir, dockerDefaultConfigDir, dockerDefaultConfigFile)
	}

	return configPath, err
}

func GetDockerConfig(configPath string) (*DockerConfig, error) {
	var dockerConfig DockerConfig

	dockerConfigPath, err := getDockerConfigPath(configPath)
	if err != nil {
		return nil, err
	}

	file, err := os.ReadFile(dockerConfigPath)
	if err != nil {
		log.Infof("Could not read docker config: %v", err)
		return nil, err
	}

	jsonError := json.Unmarshal(file, &dockerConfig)
	if jsonError != nil {
		log.Errorf("Failed to unmarshal docker config: %v", jsonError)
		return nil, jsonError
	}

	return &dockerConfig, nil
}

// GetDockerAuth extracts an auth string for the given container image name based on the credentials
// stored in docker daemon config file.
func GetDockerAuth(dockerConfig *DockerConfig, imageName string) (string, error) {
	const authStringLength = 2
	const authStringSep = ":"

	imageDomain := getImageDomainName(imageName)

	var auth DockerConfigAuth
	var ok bool
	if auth, ok = dockerConfig.Auths[imageDomain]; !ok {
		return "", nil
	}

	decodedAuth, err := base64.URLEncoding.DecodeString(auth.Auth)
	if err != nil {
		return "", err
	}

	decodedAuthSplit := strings.Split(string(decodedAuth), authStringSep)

	if len(decodedAuthSplit) != authStringLength {
		return "", errors.New("unexpected auth string")
	}

	authConfig := types.AuthConfig{
		Username: strings.TrimSpace(decodedAuthSplit[0]),
		Password: strings.TrimSpace(decodedAuthSplit[1]),
	}

	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}

	authString := base64.URLEncoding.EncodeToString(encodedJSON)
	return authString, nil
}
