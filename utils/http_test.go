package utils

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIsGitHubURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "github.com",
			input: "github.com",
			want:  true,
		},
		{
			name:  "github.com/containers/containerlab/blob/master/README.md",
			input: "github.com/containers/containerlab/blob/master/README.md",
			want:  true,
		},
		{
			name:  "google.com/containers",
			input: "google.com/containers",
			want:  false,
		},
		{
			name:  "google.com/containers/containerlab/blob/master/README.md",
			input: "google.com/containers/containerlab/blob/master/README.md",
			want:  false,
		},
		{
			name:  "gitlab.com/containers",
			input: "gitlab.com/containers",
			want:  false,
		},
		{
			name:  "raw.githubusercontent.com/containers",
			input: "raw.githubusercontent.com/containers",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if output := IsGitHubURL(tt.input); output != tt.want {
				t.Errorf("Test %q failed: want %v, but got %v", tt.name, tt.want, output)
			}
		})
	}
}

func TestGithubURLParse(t *testing.T) {
	tests := []struct {
		name           string
		ghURL          string
		expectedResult *GithubURL
		expectedError  error
	}{
		{
			name:  "bare github url without trailing slash",
			ghURL: "https://github.com/srl-labs/repo-name",
			expectedResult: &GithubURL{
				URLBase:        "https://github.com",
				ProjectOwner:   "srl-labs",
				RepositoryName: "repo-name",
			},
			expectedError: nil,
		},
		{
			name:  "bare github url with trailing slash",
			ghURL: "https://github.com/srl-labs/repo-name/",
			expectedResult: &GithubURL{
				URLBase:        "https://github.com",
				ProjectOwner:   "srl-labs",
				RepositoryName: "repo-name",
			},
			expectedError: nil,
		},
		{
			name:  "bare github url with .git suffix",
			ghURL: "https://github.com/srl-labs/repo-name.git",
			expectedResult: &GithubURL{
				URLBase:        "https://github.com",
				ProjectOwner:   "srl-labs",
				RepositoryName: "repo-name",
			},
			expectedError: nil,
		},
		{
			name:           "invalid url with just org name",
			ghURL:          "https://github.com/srl-labs/",
			expectedResult: &GithubURL{},
			expectedError:  errInvalidGithubURL,
		},
		{
			name:           "invalid url with no owner and no org",
			ghURL:          "https://github.com/",
			expectedResult: &GithubURL{},
			expectedError:  errInvalidGithubURL,
		},
		{
			name:  "github url with a clab file on the main branch",
			ghURL: "https://github.com/srl-labs/repo-name/blob/main/file.clab.yml",
			expectedResult: &GithubURL{
				URLBase:        "https://github.com",
				ProjectOwner:   "srl-labs",
				RepositoryName: "repo-name",
				GitBranch:      "main",
				FileName:       "file.clab.yml",
			},
			expectedError: nil,
		},
		{
			name:  "github url with a yaml file on the main branch",
			ghURL: "https://github.com/srl-labs/repo-name/blob/main/file.yaml",
			expectedResult: &GithubURL{
				URLBase:        "https://github.com",
				ProjectOwner:   "srl-labs",
				RepositoryName: "repo-name",
				GitBranch:      "main",
				FileName:       "file.yaml",
			},
			expectedError: nil,
		},
		{
			name:           "utl with invalid file on the main branch",
			ghURL:          "https://github.com/srl-labs/repo-name/blob/main/file.foo",
			expectedResult: &GithubURL{},
			expectedError:  errInvalidGithubURL,
		},
		{
			name:  "github url with a specified git ref and no file",
			ghURL: "https://github.com/srl-labs/repo-name/tree/some-branch",
			expectedResult: &GithubURL{
				URLBase:        "https://github.com",
				ProjectOwner:   "srl-labs",
				RepositoryName: "repo-name",
				GitBranch:      "some-branch",
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewGithubURL()
			err := u.Parse(tt.ghURL)

			if err != nil && tt.expectedError == nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if err == nil && tt.expectedError != nil {
				t.Errorf("expected error: %v, but got nil", tt.expectedError)
			}

			if err != nil && tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf("expected error: %v, but got %v", err, tt.expectedError)
				}
				// exit the test case as we don't want to compare url structs
				// since when error is available and matches the expected error
				// we don't care about the state the struct is in
				return
			}

			if diff := cmp.Diff(u, tt.expectedResult); diff != "" {
				t.Errorf("got result: = %v, expected %v, diff:\n%s", u, tt.expectedResult, diff)
			}
		})
	}
}
