package utils

import (
	"bufio"
	"os"
	"strings"
)

func IsKernelModuleLoaded(name string) (bool, error) {
	f, err := os.Open("/proc/modules")
	if err != nil {
		return false, err
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(strings.Fields(scanner.Text())[0], name) {
			return true, nil
		}
	}
	return false, f.Close()
}
