package util

import (
	"os"
	"strconv"
)

// GetEnvIntOrDefault returns the value of the environment variable k as an int *or* the default d
// if casting fails or the environment variable is not set.
func GetEnvIntOrDefault(k string, d int) int {
	if v, ok := os.LookupEnv(k); ok {
		ev, err := strconv.Atoi(v)
		if err != nil {
			return d
		}

		return ev
	}

	return d
}

// GetEnvStrOrDefault returns the value of the environment variable k as a string *or* the default d
// if casting fails or the environment variable is not set.
func GetEnvStrOrDefault(k, d string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}

	return d
}
