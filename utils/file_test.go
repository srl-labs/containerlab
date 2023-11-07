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

func TestIsHttpURL(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		allowSchemaless bool
		want            bool
	}{
		{
			name:            "Valid HTTP URL",
			url:             "http://example.com",
			allowSchemaless: false,
			want:            true,
		},
		{
			name:            "Valid HTTPS URL",
			url:             "https://example.com",
			allowSchemaless: false,
			want:            true,
		},
		{
			name:            "Valid URL without scheme",
			url:             "srlinux.dev/clab-srl",
			allowSchemaless: true,
			want:            true,
		},
		{
			name:            "Valid URL without scheme and schemaless not allowed",
			url:             "srlinux.dev/clab-srl",
			allowSchemaless: false,
			want:            false,
		},
		{
			name:            "Invalid URL",
			url:             "/foo/bar",
			allowSchemaless: false,
			want:            false,
		},
		{
			name:            "stdin symbol '-'",
			url:             "-",
			allowSchemaless: false,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHttpURL(tt.url, tt.allowSchemaless); got != tt.want {
				t.Errorf("IsHttpUri() = %v, want %v", got, tt.want)
			}
		})
	}
}
