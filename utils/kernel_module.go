package utils

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"golang.org/x/sys/unix"
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

const kernelOSReleasePath = "/proc/sys/kernel/osrelease"

// GetKernelVersion returns the parsed OS kernel version.
func GetKernelVersion() (*KernelVersion, error) {
	ver, err := os.ReadFile(kernelOSReleasePath)
	if err != nil {
		return nil, err
	}

	log.Debugf("kernel version: %s", string(ver))

	return parseKernelVersion(ver)
}

// KernelVersion holds the parsed OS kernel version.
type KernelVersion struct {
	Major     int
	Minor     int
	Revision  int
	Remainder string // the rest of the version string, e.g. "-amd64"
}

func parseKernelVersion(v []byte) (*KernelVersion, error) {
	// https://regex101.com/r/cWqad0/1
	re := regexp.MustCompile(`(?P<Major>\d+)\.(?P<Minor>\d+)\.(?P<Revision>\d+)(?P<Remainder>.*)`)

	matches := re.FindSubmatch(v)

	if len(matches) > 0 {
		kv := &KernelVersion{}

		kv.Major, _ = strconv.Atoi(string(matches[re.SubexpIndex("Major")]))
		kv.Minor, _ = strconv.Atoi(string(matches[re.SubexpIndex("Minor")]))
		kv.Revision, _ = strconv.Atoi(string(matches[re.SubexpIndex("Revision")]))
		kv.Remainder = string(matches[re.SubexpIndex("Remainder")])

		return kv, nil
	}

	return nil, fmt.Errorf("failed to parse kernel version")
}

// String returns the Kernel version as string.
func (kv *KernelVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", kv.Major, kv.Minor, kv.Revision)
}

// GreaterOrEqual returns true if the Kernel version is greater or equal to the compared Kernel version.
func (kv *KernelVersion) GreaterOrEqual(cmpKv *KernelVersion) bool {
	if kv.Major < cmpKv.Major {
		return false
	} else if kv.Major > cmpKv.Major {
		return true
	}

	if kv.Minor < cmpKv.Minor {
		return false
	} else if kv.Minor > cmpKv.Minor {
		return true
	}
	// this must be >= because we're implementing GreaterEqual
	// and this is the last position
	if kv.Revision < cmpKv.Revision {
		return false
	}

	return true
}

// ModInitFunc supports uncompressed files and gzip and xz compressed files.
func ModInitFunc(path, params string, _ int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close() // skipcq: GO-S2307

	switch filepath.Ext(path) {
	case ".gz":
		rd, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer rd.Close()

		return initModule(rd, params)
	case ".xz":
		rd, err := xz.NewReader(f)
		if err != nil {
			return err
		}

		return initModule(rd, params)
	case ".zst":
		rd, err := zstd.NewReader(f)
		if err != nil {
			return err
		}
		defer rd.Close()

		return initModule(rd, params)
	}

	// uncompressed file, first try finitModule then initModule
	if err := finitModule(int(f.Fd()), params); err != nil {
		if err == unix.ENOSYS {
			return initModule(f, params)
		}
	}

	return nil
}

// finitModule inserts a module file via syscall finit_module(2).
func finitModule(fd int, params string) error {
	return unix.FinitModule(fd, params, 0)
}

// initModule inserts a module via syscall init_module(2).
func initModule(rd io.Reader, params string) error {
	buf, err := io.ReadAll(rd)
	if err != nil {
		return err
	}
	return unix.InitModule(buf, params)
}
