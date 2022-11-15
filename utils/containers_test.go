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
