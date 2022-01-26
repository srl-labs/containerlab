package docker

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/srl-labs/containerlab/utils"
)

type imageDomainNameTest struct {
	imageName, want string
}

var imageDomainNameTests = []imageDomainNameTest{
	{imageName: "alpine", want: "docker.io"},
	{imageName: "docker.io/alpine:3.14", want: "docker.io"},
	{imageName: "example.com/example/alpine", want: "example.com"},
}

func TestGetImageDomainName(t *testing.T) {
	for _, test := range imageDomainNameTests {
		if got := GetImageDomainName(test.imageName); got != test.want {
			t.Errorf("Image domain names do not match, got %v, want %v", got, test.want)
		}
	}
}

func TestGetDockerConfigPath(t *testing.T) {
	want := "/some/path/config.json"
	got, _ := GetDockerConfigPath(want)

	if got != want {
		t.Errorf("Invalid docker config path, got %v, want %v", got, want)
	}
}

func TestGetDockerAuthContainsExpectedUser(t *testing.T) {
	// Verify that the resulting auth string contains the expected user for the given domain
	imageName := utils.GetCanonicalImageName("test.example.com/repository/alpine")
	dockerConfig, _ := GetDockerConfig("test_data/docker.config")

	want := "testuser1"

	got, err := GetDockerAuth(dockerConfig, imageName)

	if err != nil {
		t.Errorf("Error gettin docker auth, %v", err)
	}

	if err != nil {
		t.Errorf("Error decodeing auth string, error %v", err)
	}

	decodedAuthString, err := base64.URLEncoding.DecodeString(got)
	contains := strings.Contains(string(decodedAuthString), want)

	if contains != true {
		t.Errorf("Invalid docker auth, %v does not contain %v", string(decodedAuthString), want)
	}
}

func TestGetDockerAuthEmpty(t *testing.T) {
	imageName := utils.GetCanonicalImageName("alpine")
	dockerConfig, _ := GetDockerConfig("test_data/docker.config")
	want := ""

	got, err := GetDockerAuth(dockerConfig, imageName)

	if err != nil {
		t.Errorf("Error gettin docker auth, %v", err)
	}

	if err != nil {
		t.Errorf("Error decodeing auth string, error %v", err)
	}

	if got != want {
		t.Errorf("Expected empty auth string, got %v, want %v", got, want)
	}
}

func TestGetDockerAuthEmptyGivenInvalidDockerConfig(t *testing.T) {
	got, _ := GetDockerConfig("test_data/invalid_docker.config")

	if got != nil {
		t.Errorf("Expected empty auth string, got %v, want %v", got, nil)
	}
}
