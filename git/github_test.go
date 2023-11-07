package git

import (
	"errors"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// urlFromStr is a helper function to create a url.URL from a string.
func urlFromStr(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestNewGitHubRepoFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		repo    *GitHubRepo
		wantErr bool
	}{
		{
			name: "valid github url without trailing slash and with https schema",
			url:  "https://github.com/hellt/clab-test-repo",
			repo: &GitHubRepo{
				GitRepoStruct{
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
				},
			},
			wantErr: false,
		},
		{
			name: "valid github url with trailing slash and with https schema",
			url:  "https://github.com/hellt/clab-test-repo/",
			repo: &GitHubRepo{
				GitRepoStruct{
					URL:            urlFromStr("https://github.com/hellt/clab-test-repo/"),
					CloneURL:       urlFromStr("https://github.com/hellt/clab-test-repo"),
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
				},
			},
			wantErr: false,
		},
		{
			name: "valid github.dev url without trailing slash and with https schema",
			url:  "https://github.dev/hellt/clab-test-repo",
			repo: &GitHubRepo{
				GitRepoStruct{
					// github.dev links can be cloned using github.com
					URL:            urlFromStr("https://github.com/hellt/clab-test-repo"),
					CloneURL:       urlFromStr("https://github.com/hellt/clab-test-repo"),
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
				},
			},
			wantErr: false,
		},
		{
			name: "valid github.dev url with trailing slash and with https schema",
			url:  "https://github.dev/hellt/clab-test-repo/",
			repo: &GitHubRepo{
				GitRepoStruct{
					// github.dev links can be cloned using github.com
					URL:            urlFromStr("https://github.com/hellt/clab-test-repo/"),
					CloneURL:       urlFromStr("https://github.com/hellt/clab-test-repo"),
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid github url with just one path element and https schema and trailing slash",
			url:     "https://github.com/hellt/",
			wantErr: true,
		},
		{
			name: "valid github url with https schema and with .git suffix",
			url:  "https://github.com/hellt/clab-test-repo.git",
			repo: &GitHubRepo{
				GitRepoStruct{
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
				},
			},
			wantErr: false,
		},
		{
			name: "github url with a clab file on the main branch",
			url:  "https://github.com/hellt/clab-test-repo/blob/main/lab1.clab.yml",
			repo: &GitHubRepo{
				GitRepoStruct{
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
					CloneURL:       urlFromStr("https://github.com/hellt/clab-test-repo"),
					GitBranch:      "main",
					FileName:       "lab1.clab.yml",
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid file in a github url with on the main branch",
			url:     "https://github.com/hellt/clab-test-repo/blob/main/lab1.foo",
			wantErr: true,
		},
		{
			name: "github url with a clab file on the main branch in a nested dir",
			url:  "https://github.com/hellt/clab-test-repo/blob/main/dir/lab3.clab.yml",
			repo: &GitHubRepo{
				GitRepoStruct{
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
					CloneURL:       urlFromStr("https://github.com/hellt/clab-test-repo"),
					GitBranch:      "main",
					Path:           []string{"dir"},
					FileName:       "lab3.clab.yml",
				},
			},
			wantErr: false,
		},
		{
			name: "github url pointing to a branch",
			url:  "https://github.com/hellt/clab-test-repo/tree/branch1",
			repo: &GitHubRepo{
				GitRepoStruct{
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
					CloneURL:       urlFromStr("https://github.com/hellt/clab-test-repo"),
					GitBranch:      "branch1",
				},
			},
			wantErr: false,
		},
		{
			name: "github url pointing to a file in a branch",
			url:  "https://github.com/hellt/clab-test-repo/blob/branch1/lab2.clab.yml",
			repo: &GitHubRepo{
				GitRepoStruct{
					ProjectOwner:   "hellt",
					RepositoryName: "clab-test-repo",
					CloneURL:       urlFromStr("https://github.com/hellt/clab-test-repo"),
					GitBranch:      "branch1",
					FileName:       "lab2.clab.yml",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := NewGitHubRepoFromURL(urlFromStr(tt.url))

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGitURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if (err != nil) && errors.Is(err, errInvalidURL) {
				return
			}

			// when we do not manipulate url path in the ParseGitURL method
			// the wantRepo.URL field will be nil, so we need to set it up
			// to match the original URL
			if tt.repo.URL == nil {
				tt.repo.URL = urlFromStr(tt.url)
			}

			// when we do not manipulate CloneURL in the ParseGitURL method
			// the wantRepo.CloneURL field will be nil, so we need to set it up
			// to match the original URL
			if tt.repo.CloneURL == nil {
				tt.repo.CloneURL = urlFromStr(tt.url)
			}

			if diff := cmp.Diff(repo, tt.repo); diff != "" {
				t.Errorf("TestNewGitLabRepoFromURL() mismatch:\n%s", diff)
			}
		})
	}
}

func TestIsGitHubURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "github.com",
			input: "http://github.com",
			want:  true,
		},
		{
			name:  "github.com/containers/containerlab/blob/master/README.md",
			input: "https://github.com/containers/containerlab/blob/master/README.md",
			want:  true,
		},
		{
			name:  "google.com/containers",
			input: "http://google.com/containers",
			want:  false,
		},
		{
			name:  "google.com/containers/containerlab/blob/master/README.md",
			input: "https://google.com/containers/containerlab/blob/master/README.md",
			want:  false,
		},
		{
			name:  "gitlab.com/containers",
			input: "http://gitlab.com/containers",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if output := IsGitHubURL(urlFromStr(tt.input)); output != tt.want {
				t.Errorf("Test %q failed: want %v, but got %v", tt.name, tt.want, output)
			}
		})
	}
}

func TestIsGitHubShortURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "Valid Short URL",
			url:  "user/repo",
			want: true,
		},
		{
			name: "Invalid Short URL - More than one slash",
			url:  "user/repo/extra",
			want: false,
		},
		{
			name: "Invalid Short URL - Starts with http",
			url:  "http://user/repo",
			want: false,
		},
		{
			name: "normal url in short form",
			url:  "srlinux.dev/clab-srl",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitHubShortURL(tt.url); got != tt.want {
				t.Errorf("IsGitHubShortURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
