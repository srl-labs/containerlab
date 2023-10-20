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

func TestGetYAMLOrGitSuffix(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "valid suffix .yml",
			url:  "https://example.com/config.yml",
			want: ".yml",
		},
		{
			name: "valid suffix .yaml",
			url:  "https://example.com/config.yaml",
			want: ".yaml",
		},
		{
			name: "valid suffix .git",
			url:  "https://example.com/repo.git",
			want: ".git",
		},
		{
			name: "empty suffix",
			url:  "https://example.com/",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetYAMLOrGitSuffix(tt.url)

			if got != tt.want {
				t.Errorf("HasSupportedSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}
