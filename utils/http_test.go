package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestIsGithubURL(t *testing.T) {
	// tests that github urls are detected
	var tests = []struct {
		input    string
		expected bool
	}{
		{"github.com", true},
		{"github.com/containers/containerlab/blob/master/README.md", true},
		{"google.com/containers", false},
		{"google.com/containers/containerlab/blob/master/README.md", false},
		{"gitlab.com/containers", false},
		{"raw.githubusercontent.com/containers", true},
	}
	for _, test := range tests {
		if output := IsGitHubURL(test.input); output != test.expected {
			t.Error("Test Failed: {} inputted, {} expected, recieved: {}", test.input, test.expected, output)
		}
	}
}

func TestGetRawURL(t *testing.T) {
	// tests that github urls are converted to raw urls evething else is left as is
	var tests = []struct {
		input    string
		expected string
	}{
		{"github.com", "raw.githubusercontent.com"},
		{"github.com/containers/containerlab/blob/master/README.md", "raw.githubusercontent.com/containers/containerlab/master/README.md"},
		{"google.com/containers", "google.com/containers"},
		{"google.com/containers/containerlab/blob/master/README.md", "google.com/containers/containerlab/blob/master/README.md"},
		{"gitlab.com/containers", "gitlab.com/containers"},
		{"raw.githubusercontent.com/containers", "raw.githubusercontent.com/containers"},
	}
	for _, test := range tests {
		if output := GetRawURL(test.input); output != test.expected {
			t.Error("Test Failed: {} inputted, {} expected, recieved: {}", test.input, test.expected, output)
		}
	}
}

func TestCheckSuffix(t *testing.T) {
	// tests for valid suffix
    var tests = []struct {
        input    string
        expected error
    }{
        {"github.com", ErrInvalidSuffix},
        {"github.com/containers/containerlab/blob/master/README.md", ErrInvalidSuffix},
        {"google.com/containers", ErrInvalidSuffix},
        {"google.com/containers/containerlab/blob/master/README.md", ErrInvalidSuffix},
        {"gitlab.com/containers", ErrInvalidSuffix},
        {"raw.githubusercontent.com/containers", ErrInvalidSuffix},
        {"github.com/containers/containerlab/blob/master/README.yml", nil},
        {"github.com/containers/containerlab/blob/master/README.yaml", nil},
    }
    for _, test := range tests {
        if output := CheckSuffix(test.input); output != test.expected {
            t.Error("Test Failed: {} inputted, {} expected, recieved: {}", test.input, test.expected, output)
        }
    }
}
