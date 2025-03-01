package utils

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

func UnixGroupExists(groupName string) (bool, error) {
	cmd := exec.Command("getent", "group", groupName)
	out, err := cmd.Output()

	if err != nil {
		return false, fmt.Errorf("error while looking up user groups using getent command %v: %v", groupName, err)
	}

	if string(out) == "" {
		return false, nil
	}

	return true, nil
}

func getUnixGroupMembers(groupName string) ([]string, error) {
	var users []string
	cmd := exec.Command("getent", "group", groupName)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error while looking up user groups using getent command %v: %w", groupName, err)
	}

	// output format is `username:x:uid:users,comma,separated`
	// we need to extract the users, also trim the newline from the end of the output
	parts := strings.Split(strings.TrimSuffix(string(out), "\n"), ":")
	if len(parts) < 4 {
		return nil, fmt.Errorf("error while looking up user groups using getent command %v: unexpected output format", groupName)
	}

	users = strings.Split(parts[3], ",")

	return users, nil
}

func UserInUnixGroup(username, groupName string) (bool, error) {
	groupMembers, err := getUnixGroupMembers(groupName)
	if err != nil {
		return false, err
	}

	if slices.Contains(groupMembers, username) {
		return true, nil
	}

	return false, nil
}
