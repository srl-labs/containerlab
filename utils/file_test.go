// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testDownloadPaths struct {
	dir string
}

func (t testDownloadPaths) ClabTmpDir() string {
	return t.dir
}

func (t testDownloadPaths) DownloadFileTmpAbsPath(nodeName string, filenamePostfix string) string {
	return filepath.Join(t.dir, nodeName+"-"+filenamePostfix)
}

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
			if got := FilenameForURL(context.Background(), tt.args.rawUrl); got != tt.want {
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
			name:            "Valid URL without schema",
			url:             "srlinux.dev/clab-srl",
			allowSchemaless: true,
			want:            true,
		},
		{
			name:            "Valid URL without schema and schemaless not allowed",
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

func TestIsS3URL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "Valid S3 URL",
			url:  "s3://bucket/key/to/file.yaml",
			want: true,
		},
		{
			name: "Valid S3 URL with subdirectories",
			url:  "s3://my-bucket/path/to/deep/file.cfg",
			want: true,
		},
		{
			name: "HTTP URL should not match",
			url:  "https://example.com/file.yaml",
			want: false,
		},
		{
			name: "Local file path should not match",
			url:  "/path/to/file.yaml",
			want: false,
		},
		{
			name: "Empty string should not match",
			url:  "",
			want: false,
		},
		{
			name: "S3 without bucket/key should match",
			url:  "s3://",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsS3URL(tt.url); got != tt.want {
				t.Errorf("IsS3URL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDownloadableURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "HTTP URL",
			url:  "http://example.com/file.cfg",
			want: true,
		},
		{
			name: "HTTPS URL",
			url:  "https://example.com/file.cfg",
			want: true,
		},
		{
			name: "S3 URL",
			url:  "s3://bucket/file.cfg",
			want: true,
		},
		{
			name: "FTP URL",
			url:  "ftp://user:pass@example.com/file.cfg",
			want: true,
		},
		{
			name: "SCP URL",
			url:  "scp://user@example.com/file.cfg",
			want: true,
		},
		{
			name: "Local path",
			url:  "/path/to/file.cfg",
			want: false,
		},
		{
			name: "Schemaless URL",
			url:  "example.com/file.cfg",
			want: false,
		},
		{
			name: "Stdin marker",
			url:  "-",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDownloadableURL(tt.url); got != tt.want {
				t.Errorf("IsDownloadableURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessDownloadableAndEmbeddedFile(t *testing.T) {
	t.Run("embedded file", func(t *testing.T) {
		paths := testDownloadPaths{dir: t.TempDir()}
		content := "license\nblob\n"

		got, err := ProcessDownloadableAndEmbeddedFile(
			context.Background(),
			"node1",
			content,
			"embedded.lic",
			paths,
		)
		if err != nil {
			t.Fatalf("ProcessDownloadableAndEmbeddedFile() error = %v", err)
		}

		wantPath := filepath.Join(paths.dir, "node1-embedded.lic")
		if got != wantPath {
			t.Fatalf("ProcessDownloadableAndEmbeddedFile() = %q, want %q", got, wantPath)
		}

		gotContent, err := os.ReadFile(got)
		if err != nil {
			t.Fatalf("failed to read embedded file: %v", err)
		}
		if diff := cmp.Diff(content, string(gotContent)); diff != "" {
			t.Fatalf("embedded file content diff: %s", diff)
		}
	})

	t.Run("local file reference", func(t *testing.T) {
		paths := testDownloadPaths{dir: t.TempDir()}
		fileRef := "license.txt"

		got, err := ProcessDownloadableAndEmbeddedFile(
			context.Background(),
			"node1",
			fileRef,
			"embedded.lic",
			paths,
		)
		if err != nil {
			t.Fatalf("ProcessDownloadableAndEmbeddedFile() error = %v", err)
		}
		if got != fileRef {
			t.Fatalf("ProcessDownloadableAndEmbeddedFile() = %q, want %q", got, fileRef)
		}
	})
}

func TestParseS3URL(t *testing.T) {
	tests := []struct {
		name       string
		s3URL      string
		wantBucket string
		wantKey    string
		wantErr    bool
	}{
		{
			name:       "Valid S3 URL",
			s3URL:      "s3://my-bucket/path/to/file.yaml",
			wantBucket: "my-bucket",
			wantKey:    "path/to/file.yaml",
			wantErr:    false,
		},
		{
			name:       "Valid S3 URL with single file",
			s3URL:      "s3://bucket/file.cfg",
			wantBucket: "bucket",
			wantKey:    "file.cfg",
			wantErr:    false,
		},
		{
			name:       "Invalid - not an S3 URL",
			s3URL:      "https://example.com/file",
			wantBucket: "",
			wantKey:    "",
			wantErr:    true,
		},
		{
			name:       "Invalid - missing bucket",
			s3URL:      "s3:///file.yaml",
			wantBucket: "",
			wantKey:    "",
			wantErr:    true,
		},
		{
			name:       "Invalid - missing key",
			s3URL:      "s3://bucket/",
			wantBucket: "",
			wantKey:    "",
			wantErr:    true,
		},
		{
			name:       "Invalid - missing both bucket and key",
			s3URL:      "s3://",
			wantBucket: "",
			wantKey:    "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBucket, gotKey, err := ParseS3URL(tt.s3URL)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseS3URL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBucket != tt.wantBucket {
				t.Errorf("ParseS3URL() gotBucket = %v, want %v", gotBucket, tt.wantBucket)
			}
			if gotKey != tt.wantKey {
				t.Errorf("ParseS3URL() gotKey = %v, want %v", gotKey, tt.wantKey)
			}
		})
	}
}
