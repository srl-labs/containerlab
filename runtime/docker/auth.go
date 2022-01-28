package docker

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
)

type DockerConfigAuth struct {
	Auth string
}

type DockerConfig struct {
	Auths map[string]DockerConfigAuth
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
	const dockerDefaultConfigDir = ".docker"
	const dockerDefaultConfigFile = "config.json"
	var dockerConfigError error
	var dockerConfigPath string

	if configPath == "" {
		homeDir, err := os.UserHomeDir()

		if err != nil {
			dockerConfigError = err
		}

		dockerConfigPath = path.Join(homeDir, dockerDefaultConfigDir, dockerDefaultConfigFile)
	} else {
		dockerConfigPath = configPath
	}

	return dockerConfigPath, dockerConfigError
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

func GetDockerAuth(dockerConfig *DockerConfig, imageName string) (string, error) {
	const authStringLength = 2
	const authStringSep = ":"

	imageDomain := getImageDomainName(imageName)

	if domainConfig, ok := dockerConfig.Auths[imageDomain]; ok {
		decodedAuth, err := base64.URLEncoding.DecodeString(domainConfig.Auth)
		if err != nil {
			return "", err
		}

		decodedAuthSplit := strings.Split(string(decodedAuth), authStringSep)

		if len(decodedAuthSplit) == authStringLength {
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
	}

	return "", nil
}
