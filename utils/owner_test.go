package utils

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetOwner(t *testing.T) {
	// Save original environment variables
	origSudoUser := os.Getenv("SUDO_USER")
	origUser := os.Getenv("USER")
	
	// Cleanup function to restore original environment
	defer func() {
		if origSudoUser != "" {
			os.Setenv("SUDO_USER", origSudoUser)
		} else {
			os.Unsetenv("SUDO_USER")
		}
		if origUser != "" {
			os.Setenv("USER", origUser)
		} else {
			os.Unsetenv("USER")
		}
	}()

	tests := []struct {
		name      string
		sudoUser  string
		user      string
		want      string
	}{
		{
			name:     "sudo_user_set",
			sudoUser: "testuser",
			user:     "root",
			want:     "testuser",
		},
		{
			name:     "only_user_set",
			sudoUser: "",
			user:     "normaluser",
			want:     "normaluser",
		},
		{
			name:     "both_empty",
			sudoUser: "",
			user:     "",
			want:     "",
		},
		{
			name:     "both_set_sudo_takes_precedence",
			sudoUser: "sudouser",
			user:     "regularuser",
			want:     "sudouser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for this test
			if tt.sudoUser != "" {
				os.Setenv("SUDO_USER", tt.sudoUser)
			} else {
				os.Unsetenv("SUDO_USER")
			}
			
			if tt.user != "" {
				os.Setenv("USER", tt.user)
			} else {
				os.Unsetenv("USER")
			}

			got := GetOwner()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}