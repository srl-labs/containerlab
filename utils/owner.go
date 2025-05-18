package utils

import "os"

// GetOwner determines the owner name from a provided parameter or environment variables.
// It first checks the provided owner parameter, then falls back to the SUDO_USER
// environment variable, and finally the USER environment variable.
func GetOwner(owner string) string {
	if owner != "" {
		return owner
	}
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser
	}
	return os.Getenv("USER")
}
