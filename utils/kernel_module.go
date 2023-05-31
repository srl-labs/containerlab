package utils

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// IsKernelModuleLoaded checks if a kernel module is loaded by parsing /proc/modules file.
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

func GetKernelVersion() (*KernelVersion, error) {
	var uts syscall.Utsname
	syscall.Uname(&uts)
	return ParseKernelVersion(charsToString(uts.Release[:]))

}

func charsToString(chars []int8) string {
	var sb strings.Builder
	for _, c := range chars {
		if c == '\x00' {
			break
		}
		sb.WriteByte(byte(c))
	}
	return strings.TrimSpace(sb.String())
}

type KernelVersion struct {
	Major    int
	Minor    int
	Revision int
	Remains  string
}

func ParseKernelVersion(v string) (*KernelVersion, error) {
	var err error
	r := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)(.*)`)

	split := r.FindStringSubmatch(v)

	if len(split) > 5 && len(split) < 4 {
		return nil, fmt.Errorf("unable to parse %q as kernel version", v)
	}

	// remove the full string which is store in position [0]
	split = split[1:]

	versionParts := make([]int, 3)
	for i, v := range split[:2] {
		versionParts[i], err = strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
	}
	result := &KernelVersion{
		Major:    versionParts[0],
		Minor:    versionParts[1],
		Revision: versionParts[2],
	}
	if len(split) == 4 {
		result.Remains = split[3]
	}
	return result, nil
}

// StringMMR returns the Kernel version in <Major>.<Minor>.<Revision>
func (kv *KernelVersion) StringMMR() string {
	return fmt.Sprintf("%d.%d.%d", kv.Major, kv.Minor, kv.Revision)
}

func (kv *KernelVersion) IsGreaterEqual(cmpKv *KernelVersion) bool {
	if kv.Major < cmpKv.Major {
		return false
	}
	if kv.Minor < cmpKv.Minor {
		return false
	}
	// this must be >= because we're implementing GreaterEqual
	// and this is the last position
	if kv.Revision < cmpKv.Revision {
		return false
	}
	return true
}
