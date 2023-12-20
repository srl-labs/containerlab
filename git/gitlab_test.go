package git

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewGitLabRepoFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		repo    *GitLabRepo
		wantErr bool
	}{
		{
			name: "valid gitlab url without trailing slash and with https schema",
			url:  "https://gitlab.com/rdodin/clab-test-repo",
			repo: &GitLabRepo{
				GitRepoStruct{
					ProjectOwner:   "rdodin",
					RepositoryName: "clab-test-repo",
				},
			},
			wantErr: false,
		},
		{
			name: "valid gitlab url with trailing slash and with https schema",
			url:  "https://gitlab.com/rdodin/clab-test-repo/",
			repo: &GitLabRepo{
				GitRepoStruct{
					URL:            urlFromStr("https://gitlab.com/rdodin/clab-test-repo/"),
					CloneURL:       urlFromStr("https://gitlab.com/rdodin/clab-test-repo"),
					ProjectOwner:   "rdodin",
					RepositoryName: "clab-test-repo",
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid gitlab url with just one path element and https schema and trailing slash",
			url:     "https://gitlab.com/rdodin/",
			wantErr: true,
		},
		{
			name: "valid gitlab url with https schema and with .git suffix",
			url:  "https://gitlab.com/rdodin/clab-test-repo.git",
			repo: &GitLabRepo{
				GitRepoStruct{
					ProjectOwner:   "rdodin",
					RepositoryName: "clab-test-repo",
				},
			},
			wantErr: false,
		},
		{
			name: "gitlab url with a clab file on the main branch without path args",
			url:  "https://gitlab.com/rdodin/clab-test-repo/-/blob/main/lab1.clab.yml",
			repo: &GitLabRepo{
				GitRepoStruct{
					ProjectOwner:   "rdodin",
					RepositoryName: "clab-test-repo",
					CloneURL:       urlFromStr("https://gitlab.com/rdodin/clab-test-repo"),
					GitBranch:      "main",
					FileName:       "lab1.clab.yml",
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid file in a gitlab url with on the main branch",
			url:     "https://gitlab.com/rdodin/clab-test-repo/-/blob/main/lab1.foo",
			wantErr: true,
		},
		{
			name: "gitlab url with a clab file on the main branch in a nested dir without path args",
			url:  "https://gitlab.com/rdodin/clab-test-repo/-/blob/main/dir/lab3.clab.yml",
			repo: &GitLabRepo{
				GitRepoStruct{
					ProjectOwner:   "rdodin",
					RepositoryName: "clab-test-repo",
					CloneURL:       urlFromStr("https://gitlab.com/rdodin/clab-test-repo"),
					GitBranch:      "main",
					Path:           []string{"dir"},
					FileName:       "lab3.clab.yml",
				},
			},
			wantErr: false,
		},
		{
			name: "gitlab url pointing to a branch with a path arg",
			url:  "https://gitlab.com/rdodin/clab-test-repo/-/tree/branch1?ref_type=heads",
			repo: &GitLabRepo{
				GitRepoStruct{
					ProjectOwner:   "rdodin",
					RepositoryName: "clab-test-repo",
					CloneURL:       urlFromStr("https://gitlab.com/rdodin/clab-test-repo"),
					GitBranch:      "branch1",
				},
			},
			wantErr: false,
		},
		{
			name: "gitlab url pointing to a file in a branch",
			url:  "https://gitlab.com/rdodin/clab-test-repo/-/blob/branch1/lab2.clab.yml?ref_type=heads",
			repo: &GitLabRepo{
				GitRepoStruct{
					ProjectOwner:   "rdodin",
					RepositoryName: "clab-test-repo",
					CloneURL:       urlFromStr("https://gitlab.com/rdodin/clab-test-repo"),
					GitBranch:      "branch1",
					FileName:       "lab2.clab.yml",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := NewGitLabRepoFromURL(urlFromStr(tt.url))

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

func TestIsGitLabURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "gitlab.com",
			input: "http://gitlab.com",
			want:  true,
		},
		{
			name:  "gitlab.com/containers/containerlab/blob/master/README.md",
			input: "https://gitlab.com/containers/containerlab/blob/master/README.md",
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
			name:  "github.com/containers",
			input: "http://github.com/containers",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if output := IsGitLabURL(urlFromStr(tt.input)); output != tt.want {
				t.Errorf("Test %q failed: want %v, but got %v", tt.name, tt.want, output)
			}
		})
	}
}
