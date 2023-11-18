package git

import (
	"errors"
	"net/url"
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
			name:  "https://github.com",
			input: "https://github.com",
			want:  true,
		},
		{
			name:  "https://github.com/containers/containerlab/blob/master/README.md",
			input: "https://github.com/containers/containerlab/blob/master/README.md",
			want:  true,
		},
		{
			name:  "https://google.com/containers",
			input: "https://google.com/containers",
			want:  false,
		},
		{
			name:  "https://google.com/containers/containerlab/blob/master/README.md",
			input: "https://google.com/containers/containerlab/blob/master/README.md",
			want:  false,
		},
		{
			name:  "https://gitlab.com/containers",
			input: "https://gitlab.com/containers",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if err != nil {
				t.Errorf("failed parsing url provided in test.")
			}

			if output := IsGitHubURL(u); output != tt.want {
				t.Errorf("Test %q failed: want %v, but got %v", tt.name, tt.want, output)
			}
		})
	}
}

func TestGitHubGitRepoParse(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult *GitHubRepo
		expectedError  error
	}{
		{
			name:  "bare github url without trailing slash",
			input: "https://github.com/srl-labs/repo-name",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					Path:           nil,
					ProjectOwner:   "srl-labs",
					RepositoryName: "repo-name",
				},
			},
			expectedError: nil,
		},
		{
			name:  "bare github url with trailing slash",
			input: "https://github.com/srl-labs/repo-name/",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					Path:           nil,
					ProjectOwner:   "srl-labs",
					RepositoryName: "repo-name",
				},
			},
			expectedError: nil,
		},
		{
			name:  "bare github.dev url with trailing slash",
			input: "https://github.dev/srl-labs/repo-name/",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					Path:           nil,
					ProjectOwner:   "srl-labs",
					RepositoryName: "repo-name",
				},
			},
			expectedError: nil,
		},
		{
			name:  "bare github url with .git suffix",
			input: "https://github.com/srl-labs/repo-name.git",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					Path:           nil,
					ProjectOwner:   "srl-labs",
					RepositoryName: "repo-name",
				},
			},
			expectedError: nil,
		},
		{
			name:           "invalid url with just org name",
			input:          "https://github.com/srl-labs/",
			expectedResult: &GitHubRepo{},
			expectedError:  errInvalidURL,
		},
		{
			name:           "invalid url with no owner and no org",
			input:          "https://github.com/",
			expectedResult: &GitHubRepo{},
			expectedError:  errInvalidURL,
		},
		{
			name:  "github url with a clab file on the main branch",
			input: "https://github.com/srl-labs/repo-name/blob/main/file.clab.yml",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					Path:           []string{},
					ProjectOwner:   "srl-labs",
					RepositoryName: "repo-name",
					GitBranch:      "main",
					FileName:       "file.clab.yml",
				},
			},
			expectedError: nil,
		},
		{
			name:  "github url with a yaml file on the main branch",
			input: "https://github.com/srl-labs/repo-name/blob/main/file.yaml",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					ProjectOwner:   "srl-labs",
					RepositoryName: "repo-name",
					GitBranch:      "main",
					Path:           []string{},
					FileName:       "file.yaml",
				},
			},
			expectedError: nil,
		},
		{
			name:           "url with invalid file on the main branch",
			input:          "https://github.com/srl-labs/repo-name/blob/main/file.foo",
			expectedResult: &GitHubRepo{},
			expectedError:  errInvalidURL,
		},
		{
			name:  "github url with a specified git ref and no file",
			input: "https://github.com/srl-labs/repo-name/tree/some-branch",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					ProjectOwner:   "srl-labs",
					RepositoryName: "repo-name",
					GitBranch:      "some-branch",
					Path:           []string{},
				},
			},
			expectedError: nil,
		},
		{
			name:  "github url with a specified git ref and no file and trailing slash",
			input: "https://github.com/srl-labs/repo-name/tree/some-branch/",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					ProjectOwner:   "srl-labs",
					RepositoryName: "repo-name",
					GitBranch:      "some-branch",
					Path:           []string{},
				},
			},
			expectedError: nil,
		},
		{
			name:  "github url with ref to file in subdir",
			input: "https://github.com/srl-labs/containerlab/blob/main/lab-examples/srl01/srl01.clab.yml",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					ProjectOwner:   "srl-labs",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{"lab-examples", "srl01"},
					FileName:       "srl01.clab.yml",
				},
			},
			expectedError: nil,
		},
		{
			name:  "github url with ref to subdir",
			input: "https://github.com/srl-labs/containerlab/tree/main/lab-examples/srl01/",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					ProjectOwner:   "srl-labs",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{"lab-examples", "srl01"},
				},
			},
			expectedError: nil,
		},
		{
			name:  "github url with tree ref to repo root",
			input: "https://github.com/srl-labs/containerlab/tree/main",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					ProjectOwner:   "srl-labs",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{},
				},
			},
			expectedError: nil,
		},
		{
			name:  "github url with tree ref to file in repo root",
			input: "https://github.com/srl-labs/containerlab/blob/main/mytopo.yml",
			expectedResult: &GitHubRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "github.com",
					},
					ProjectOwner:   "srl-labs",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{},
					FileName:       "mytopo.yml",
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if err != nil {
				t.Errorf("failed parsing url provided in test.")
			}
			repo, err := ParseGitHubRepoUrl(u)

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

			if diff := cmp.Diff(repo, tt.expectedResult); diff != "" {
				t.Errorf("got result: = %v, expected %v, diff:\n%s", repo, tt.expectedResult, diff)
			}
		})
	}
}

func TestParseGitLabRepoUrl(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult *GitLabRepo
		expectedError  error
	}{
		{
			name:  "bare gitlab url without trailing slash",
			input: "https://fake.gitlab.com/user/repo-name",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					Path:           nil,
					ProjectOwner:   "user",
					RepositoryName: "repo-name",
				},
			},
			expectedError: nil,
		},
		{
			name:  "bare gitlab url with trailing slash",
			input: "https://fake.gitlab.com/user/repo-name/",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					Path:           nil,
					ProjectOwner:   "user",
					RepositoryName: "repo-name",
				},
			},
			expectedError: nil,
		},
		{
			name:  "bare github url with .git suffix",
			input: "https://fake.gitlab.com/user/repo-name.git",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					Path:           nil,
					ProjectOwner:   "user",
					RepositoryName: "repo-name",
				},
			},
			expectedError: nil,
		},
		{
			name:           "invalid url with just org name",
			input:          "https://fake.gitlab.com/user/",
			expectedResult: &GitLabRepo{},
			expectedError:  errInvalidURL,
		},
		{
			name:           "invalid url with no owner and no org",
			input:          "https:/fake.gitlab.com/",
			expectedResult: &GitLabRepo{},
			expectedError:  errInvalidURL,
		},
		{
			name:  "gitlab url with a clab file on the main branch",
			input: "https://fake.gitlab.com/user/repo-name/-/blob/main/file.clab.yml",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					Path:           []string{},
					ProjectOwner:   "user",
					RepositoryName: "repo-name",
					GitBranch:      "main",
					FileName:       "file.clab.yml",
				},
			},
			expectedError: nil,
		},
		{
			name:  "gitlab url with a yaml file on the main branch",
			input: "https://fake.gitlab.com/user/repo-name/-/blob/main/file.yaml",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					ProjectOwner:   "user",
					RepositoryName: "repo-name",
					GitBranch:      "main",
					Path:           []string{},
					FileName:       "file.yaml",
				},
			},
			expectedError: nil,
		},
		{
			name:           "url with invalid file on the main branch",
			input:          "https://fake.gitlab.com/user/repo-name/-/blob/main/file.foo",
			expectedResult: &GitLabRepo{},
			expectedError:  errInvalidURL,
		},
		{
			name:  "gitlab url with a specified git ref and no file",
			input: "https://fake.gitlab.com/user/repo-name/-/tree/some-branch",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					ProjectOwner:   "user",
					RepositoryName: "repo-name",
					GitBranch:      "some-branch",
					Path:           []string{},
				},
			},
			expectedError: nil,
		},
		{
			name:  "gitlab url with a specified git ref and no file and trailing slash",
			input: "https://fake.gitlab.com/user/repo-name/-/tree/some-branch/",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					ProjectOwner:   "user",
					RepositoryName: "repo-name",
					GitBranch:      "some-branch",
					Path:           []string{},
				},
			},
			expectedError: nil,
		},
		{
			name:  "gitlab url with ref to file in subdir",
			input: "https://fake.gitlab.com/user/containerlab/-/blob/main/lab-examples/srl01/srl01.clab.yml",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					ProjectOwner:   "user",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{"lab-examples", "srl01"},
					FileName:       "srl01.clab.yml",
				},
			},
			expectedError: nil,
		},
		{
			name:  "gitlab url with ref to subdir",
			input: "https://fake.gitlab.com/user/containerlab/-/tree/main/lab-examples/srl01/",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					ProjectOwner:   "user",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{"lab-examples", "srl01"},
				},
			},
			expectedError: nil,
		},
		{
			name:  "gitlab url with tree ref to repo root",
			input: "https://fake.gitlab.com/user/containerlab/-/tree/main",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					ProjectOwner:   "user",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{},
				},
			},
			expectedError: nil,
		},
		{
			name:  "gitlab url with tree ref to file in repo root",
			input: "https://fake.gitlab.com/user/containerlab/-/blob/main/mytopo.yml",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					ProjectOwner:   "user",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{},
					FileName:       "mytopo.yml",
				},
			},
			expectedError: nil,
		},
		{
			name:  "gitlab url with tree ref to file in repo root and query parameters",
			input: "https://fake.gitlab.com/user/containerlab/-/blob/main/mytopo.yml?foo=bar",
			expectedResult: &GitLabRepo{
				GitRepoStruct: GitRepoStruct{
					URL: url.URL{
						Scheme: "https",
						Host:   "fake.gitlab.com",
					},
					ProjectOwner:   "user",
					RepositoryName: "containerlab",
					GitBranch:      "main",
					Path:           []string{},
					FileName:       "mytopo.yml",
				},
			},
			expectedError: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if err != nil {
				t.Errorf("failed parsing url provided in test.")
			}
			repo, err := ParseGitLabRepoUrl(u)

			if err != nil && tt.expectedError == nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if err == nil && tt.expectedError != nil {
				t.Errorf("expected error: %v, but got nil", tt.expectedError)
			}

			if err != nil && tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf("expected error: %v, but got %v", tt.expectedError, err)
				}
				// exit the test case as we don't want to compare url structs
				// since when error is available and matches the expected error
				// we don't care about the state the struct is in
				return
			}

			if diff := cmp.Diff(repo, tt.expectedResult); diff != "" {
				t.Errorf("got result: = %v, expected %v, diff:\n%s", repo, tt.expectedResult, diff)
			}
		})
	}
}
