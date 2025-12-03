package utils

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetRegexpCaptureGroups(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		search  string
		want    map[string]string
		wantErr bool
		errStr  string
	}{
		{
			name:    "simple_named_groups",
			pattern: `(?P<protocol>\w+)://(?P<host>[^/]+)(?P<path>/.*)`,
			search:  "https://example.com/path/to/resource",
			want: map[string]string{
				"protocol": "https",
				"host":     "example.com",
				"path":     "/path/to/resource",
			},
			wantErr: false,
		},
		{
			name:    "partial_named_groups",
			pattern: `(?P<name>\w+)@(\w+)\.com`,
			search:  "user@example.com",
			want: map[string]string{
				"name": "user",
			},
			wantErr: false,
		},
		{
			name:    "no_named_groups",
			pattern: `(\w+)@(\w+)\.com`,
			search:  "user@example.com",
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "no_match",
			pattern: `(?P<number>\d+)`,
			search:  "no numbers here",
			want:    nil,
			wantErr: true,
			errStr:  "does not match regexp",
		},
		{
			name:    "empty_search",
			pattern: `(?P<word>\w+)`,
			search:  "",
			want:    nil,
			wantErr: true,
			errStr:  "does not match regexp",
		},
		{
			name:    "multiple_same_named_groups",
			pattern: `(?P<digit>\d).*(?P<digit>\d)`,
			search:  "1abc2",
			want: map[string]string{
				"digit": "2", // Last match wins
			},
			wantErr: false,
		},
		{
			name:    "version_parsing",
			pattern: `v(?P<major>\d+)\.(?P<minor>\d+)\.(?P<patch>\d+)`,
			search:  "v1.2.3",
			want: map[string]string{
				"major": "1",
				"minor": "2",
				"patch": "3",
			},
			wantErr: false,
		},
		{
			name:    "ip_address_parsing",
			pattern: `(?P<ip>(?P<octet1>\d{1,3})\.(?P<octet2>\d{1,3})\.(?P<octet3>\d{1,3})\.(?P<octet4>\d{1,3}))`,
			search:  "192.168.1.10",
			want: map[string]string{
				"ip":     "192.168.1.10",
				"octet1": "192",
				"octet2": "168",
				"octet3": "1",
				"octet4": "10",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := regexp.Compile(tt.pattern)
			if err != nil {
				t.Fatalf("failed to compile regex pattern %q: %v", tt.pattern, err)
			}

			got, err := GetRegexpCaptureGroups(r, tt.search)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetRegexpCaptureGroups() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if tt.errStr != "" && !strings.Contains(err.Error(), tt.errStr) {
					t.Fatalf("expected error containing %q, got %q", tt.errStr, err.Error())
				}
				return
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetRegexpCaptureGroups_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "numeric_groups_ignored",
			test: func(t *testing.T) {
				// Test that numbered groups (index 0) are ignored
				r := regexp.MustCompile(`(test)(?P<named>value)`)
				got, err := GetRegexpCaptureGroups(r, "testvalue")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				want := map[string]string{"named": "value"}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "special_characters_in_capture",
			test: func(t *testing.T) {
				r := regexp.MustCompile(`(?P<special>[!@#$%^&*()]+)`)
				got, err := GetRegexpCaptureGroups(r, "!@#$%^&*()")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				want := map[string]string{"special": "!@#$%^&*()"}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "complex_regex_with_multiple_groups",
			test: func(t *testing.T) {
				// Test with complex regex having multiple capture groups
				r := regexp.MustCompile(
					`(?P<protocol>https?)://(?P<user>\w+):(?P<pass>\w+)@(?P<host>[^/]+)/(?P<path>.*)`,
				)
				got, err := GetRegexpCaptureGroups(
					r,
					"https://user:password@example.com/path/to/resource",
				)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				want := map[string]string{
					"protocol": "https",
					"user":     "user",
					"pass":     "password",
					"host":     "example.com",
					"path":     "path/to/resource",
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Fatalf("mismatch (-want +got):\n%s", diff)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}
