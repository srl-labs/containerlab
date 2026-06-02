package utils

import "os"

// GetOwner determines the lab owner by first checking the SUDO_USER environment variable,
// and then if that is not set the USER environment variable.
func GetOwner() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return sudoUser
	}

	return os.Getenv("USER")
}
