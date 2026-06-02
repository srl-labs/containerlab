package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDocsLinkFromVer(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "major and minor only",
			version:  "0.47.0",
			expected: "0.47/",
		},
		{
			name:     "major, minor, and patch version",
			version:  "0.47.2",
			expected: "0.47/#0472",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := docsLinkFromVer(tt.version)
			if diff := cmp.Diff(got, tt.expected); diff != "" {
				t.Fatalf("docsLinkFromVer() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
