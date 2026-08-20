package core

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/unix"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	clabHostEntryPrefix  = "###### CLAB-%s-START ######"
	clabHostEntryPostfix = "###### CLAB-%s-END ######"

	hostEntriesPerNode = 2
)

// Overridable from tests; never mutated in production.
var (
	clabHostsFilename = "/etc/hosts"
	clabHostsLockPath = "/run/lock/clab-hosts.lock"
)

var hostsFileMu sync.Mutex

// withHostsFileLock serializes /etc/hosts edits across goroutines and
// across concurrent clab processes on the same host.
func withHostsFileLock(fn func() error) error {
	hostsFileMu.Lock()
	defer hostsFileMu.Unlock()

	if dir := filepath.Dir(clabHostsLockPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating hosts lock directory %q: %w", dir, err)
		}
	}

	// 0o644 so a non-root caller can co-acquire the lock with a prior root caller.
	f, err := os.OpenFile(clabHostsLockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("opening hosts file lock %q: %w", clabHostsLockPath, err)
	}
	defer f.Close()

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring hosts file lock: %w", err)
	}

	return fn()
}

func (c *CLab) appendHostsFileEntries(ctx context.Context) error {
	return withHostsFileLock(func() error { return c.appendHostsFileEntriesLocked(ctx) })
}

func (c *CLab) appendHostsFileEntriesLocked(ctx context.Context) error {
	filename := clabHostsFilename

	if c.Config.Name == "" {
		return fmt.Errorf("missing lab name")
	}

	if !clabutils.FileExists(filename) {
		err := clabutils.CreateFile(filename, "127.0.0.1\tlocalhost")
		if err != nil {
			return err
		}
	}

	// lets make sure to remove the entries of a non-properly destroyed lab in the hosts file
	err := c.deleteEntriesFromHostsFileLocked()
	if err != nil {
		return err
	}

	hostEntries := make(clabtypes.HostEntries, 0, len(c.Nodes)*hostEntriesPerNode)
	for _, n := range c.Nodes {
		nodeHostEntries, err := n.GetHostsEntries(ctx)
		if err != nil {
			return err
		}

		hostEntries.Merge(nodeHostEntries)
	}

	var f *os.File

	f, err = os.OpenFile(clabHostsFilename, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}

	defer f.Close() // skipcq: GO-S2307

	content := &bytes.Buffer{}
	fmt.Fprintf(content, clabHostEntryPrefix, c.Config.Name)
	fmt.Fprint(content, "\n")
	fmt.Fprint(content, hostEntries.ToHostsConfig(clabtypes.IpVersionV4))
	fmt.Fprint(content, hostEntries.ToHostsConfig(clabtypes.IpVersionV6))
	fmt.Fprintf(content, clabHostEntryPostfix, c.Config.Name)
	fmt.Fprint(content, "\n")

	_, err = f.ReadFrom(content)
	if err != nil {
		return err
	}

	return nil
}

func (c *CLab) DeleteEntriesFromHostsFile() error {
	return withHostsFileLock(c.deleteEntriesFromHostsFileLocked)
}

func (c *CLab) deleteEntriesFromHostsFileLocked() error {
	if c.Config.Name == "" {
		return errors.New("missing containerlab name")
	}

	f, err := os.OpenFile(
		clabHostsFilename,
		os.O_RDWR,
		clabconstants.PermissionsFileDefault,
	) // skipcq: GSC-G302
	if err != nil {
		return err
	}

	defer f.Close() // skipcq: GO-S2307

	reader := bufio.NewReader(f)
	skiplines := false
	output := bytes.Buffer{}
	prefix := fmt.Sprintf(clabHostEntryPrefix, c.Config.Name)
	postfix := fmt.Sprintf(clabHostEntryPostfix, c.Config.Name)

	for {
		line, err := reader.ReadString(byte('\n'))
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if strings.TrimSpace(line) == postfix {
			skiplines = false
			continue
		} else if strings.TrimSpace(line) == prefix || skiplines {
			skiplines = true
			continue
		}

		output.WriteString(line)
	}

	if skiplines {
		// if skiplines is not false, we did not find the end
		// so we should not mess with /etc/hosts
		return fmt.Errorf("issue cleaning up %s file. Please do so manually", clabHostsFilename)
	}

	err = f.Truncate(0)
	if err != nil {
		return err
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = f.Write(output.Bytes())

	return err
}
