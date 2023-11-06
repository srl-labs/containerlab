// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFilenameForURL(t *testing.T) {
	type args struct {
		rawUrl string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "regular filename url",
			args: args{
				rawUrl: "http://myserver.foo/download/raw/node1.cfg",
			},
			want: "node1.cfg",
		},
		{
			name: "folder URL",
			args: args{
				rawUrl: "http://myserver.foo/download/raw/",
			},
			want: "raw",
		},
		{
			name: "with get parameters",
			args: args{
				rawUrl: "http://myserver.foo/download/raw/node1.cfg?foo=bar&bar=foo",
			},
			want: "node1.cfg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilenameForURL(tt.args.rawUrl); got != tt.want {
				t.Errorf("got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestFileLines(t *testing.T) {
	type args struct {
		path       string
		commentStr string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "regular file",
			args: args{
				path:       "test_data/keys1.txt",
				commentStr: "#",
			},
			want:    []string{"valid line", "another valid line"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FileLines(tt.args.path, tt.args.commentStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileLines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("FileLines() diff = %s", d)
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
		// Add more test cases as needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitHubShortURL(tt.url); got != tt.want {
				t.Errorf("IsGitHubShortURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
