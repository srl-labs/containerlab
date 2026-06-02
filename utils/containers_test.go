package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetCanonicalImageName(t *testing.T) {
	tests := map[string]struct {
		got  string
		want string
	}{
		"short dockerhub name no tag": {
			got:  "alpine",
			want: "docker.io/library/alpine:latest",
		},
		"long dockerhub name no tag": {
			got:  "linux/alpine",
			want: "docker.io/linux/alpine:latest",
		},
		"short dockerhub name and latest tag": {
			got:  "alpine:latest",
			want: "docker.io/library/alpine:latest",
		},
		"long dockerhub name latest tag": {
			got:  "linux/alpine:latest",
			want: "docker.io/linux/alpine:latest",
		},
		"short non-dockerhub name and no tag": {
			got:  "custom.io/alpine",
			want: "custom.io/alpine:latest",
		},
		"long non-dockerhub name and no tag": {
			got:  "custom.io/linux/alpine",
			want: "custom.io/linux/alpine:latest",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := GetCanonicalImageName(tc.got)

			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %q, got %q", tc.want, got)
			}
		})
	}
}

func TestDestinationBindMountExists(t *testing.T) {
	tests := map[string]struct {
		binds []string
		dest  string
		want  bool
	}{
		"empty binds slice": {
			binds: []string{},
			dest:  "/target",
			want:  false,
		},
		"single bind with matching destination": {
			binds: []string{"/source:/target"},
			dest:  "/target",
			want:  true,
		},
		"multiple binds with one matching destination": {
			binds: []string{"/source1:/target1", "/source2:/target"},
			dest:  "/target",
			want:  true,
		},
		"no matching destination": {
			binds: []string{"/source1:/target1", "/source2:/target2"},
			dest:  "/target3",
			want:  false,
		},
		"bind with additional options": {
			binds: []string{"/source:/target:ro,z"},
			dest:  "/target",
			want:  true,
		},
		"malformed bind without separator": {
			binds: []string{"malformedstring"},
			dest:  "/target",
			want:  false,
		},
		"bind with empty parts": {
			binds: []string{":/target:", "/source:"},
			dest:  "/target",
			want:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := DestinationBindMountExists(tc.binds, tc.dest)
			if !cmp.Equal(got, tc.want) {
				t.Fatalf("wanted %v, got %v", tc.want, got)
			}
		})
	}
}
