package utils

import (
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


func TestHasSupportedSuffix(t *testing.T) {
	// tests that supported suffixes are detected
	var tests = []struct {
		input    string
		expected string
	}{
		{"google.com/containers/containerlab/blob/master/README.md", ""},
		{"gitlab.com/containers", ""},
		{"raw.githubusercontent.com/containers.git", ".git"},
		{"github.com/containers/containerlab/blob/master/README.yml", ".yml"},
		{"github.com/containers/containerlab/blob/master/README.yaml", ".yaml"},
		{"github.com/containers/containerlab/blob/master/README.txt", ""},
	}
	for _, test := range tests {
		if output, _ := HasSupportedSuffix(test.input); output != test.expected {
			t.Error("Test Failed: {} inputted, {} expected, recieved: {}", test.input, test.expected, output)
		}
	}
}