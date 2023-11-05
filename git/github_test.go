package git

import (
	"errors"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func urlFromStr(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestNewGitLabRepoFromURL(t *testing.T) {
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
