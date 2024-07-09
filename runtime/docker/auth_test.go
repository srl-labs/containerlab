package docker

import (
	"testing"

	"github.com/mitchellh/go-homedir"
	"github.com/srl-labs/containerlab/utils"
)

type imageDomainNameTest struct {
	imageName, want string
}

var imageDomainNameTests = []imageDomainNameTest{
	{imageName: "alpine", want: "docker.io"},
	{imageName: "docker.io/alpine:3.14", want: "docker.io"},
	{imageName: "example.com/example/alpine", want: "example.com"},
	{imageName: ".invalid_format", want: ""},
	{imageName: "", want: ""},
}

var authTests = map[string]struct {
	ConfigPath         string
	Image              string
	ExpectedAuthString string
	ExpectedErr        bool
}{
	"valid-config-valid-auth-data": {
		ConfigPath:         "test_data/docker.config",
		Image:              "test.example.com/repository/alpine",
		ExpectedAuthString: "eyJ1c2VybmFtZSI6InRlc3R1c2VyMSIsInBhc3N3b3JkIjoidGVzdHBhc3MxIn0=",
		ExpectedErr:        false,
	},
	"valid-config-invalid-image-name": {
		ConfigPath:         "test_data/docker.config",
		Image:              "some.wrong/repo/image",
		ExpectedAuthString: "",
		ExpectedErr:        false,
	},
}

func TestGetImageDomainName(t *testing.T) {
	for _, test := range imageDomainNameTests {
		if got := getImageDomainName(test.imageName); got != test.want {
			t.Errorf("Image domain names do not match, got %v, want %v", got, test.want)
		}
	}
}

func TestGetDockerConfigPath(t *testing.T) {
	td := map[string]map[string]string{
		"fixed-path": {
			"path": "/some/path/config.json",
			"want": "/some/path/config.json",
		},
		"default": {
			"path": "",
			"want": "~/.docker/config.json",
		},
	}

	for _, in := range td {
		got := getDockerConfigPath(in["path"])
		want, _ := homedir.Expand(in["want"])
		if got != want {
			t.Errorf("Invalid docker config path, got %v, want %v", got, in["want"])
		}
	}
}

func TestGetDockerAuth(t *testing.T) {
	for _, data := range authTests {
		img := utils.GetCanonicalImageName(data.Image)
		cfg, _ := GetDockerConfig(data.ConfigPath)

		auth, err := GetDockerAuth(cfg, img)
		if err != nil {
			t.Error(err)
		}

		t.Logf("auth string: %v", auth)

		if auth != data.ExpectedAuthString {
			t.Errorf("expected auth string '%s' does not match computed '%s'", data.ExpectedAuthString, auth)
		}

	}
}
