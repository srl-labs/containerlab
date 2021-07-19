package clab

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cloudflare/cfssl/log"
	"github.com/srl-labs/containerlab/types"
)

var (
	CLAB_HOSTENTRY_PREFIX  = "###### CLAB-%s-START ######"
	CLAB_HOSTENTRY_POSTFIX = "###### CLAB-%s-END ######"
)

func AppendHostsFileEntries(containers []types.GenericContainer, labname string) error {
	if labname == "" {
		return fmt.Errorf("missing lab name")
	}
	// lets make sure we do not have remaining of a non destroyed run in the hosts file
	err := DeleteEntriesFromHostsFile(labname)
	if err != nil {
		return err
	}
	data := GenerateHostsEntries(containers, labname)
	if len(data) == 0 {
		return nil
	}
	f, err := os.OpenFile("/etc/hosts", os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\n")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// hostEntries builds an /etc/hosts compliant text blob (as []byte]) for containers ipv4/6 address<->name pairs
func GenerateHostsEntries(containers []types.GenericContainer, labname string) []byte {
	entries := strings.Builder{}
	v6entries := strings.Builder{}

	entries.WriteString(fmt.Sprintf(CLAB_HOSTENTRY_PREFIX+"\n", labname))

	for _, cont := range containers {
		if len(cont.Names) == 0 {
			continue
		}
		if cont.NetworkSettings.Set {
			if cont.NetworkSettings.IPv4addr != "" {
				fmt.Fprintf(&entries, "%s\t%s\n", cont.NetworkSettings.IPv4addr, strings.TrimLeft(cont.Names[0], "/"))
			}
			if cont.NetworkSettings.IPv6addr != "" {
				fmt.Fprintf(&v6entries, "%s\t%s\n", cont.NetworkSettings.IPv6addr, strings.TrimLeft(cont.Names[0], "/"))
			}
		}
	}

	entries.WriteString(v6entries.String())
	entries.WriteString(fmt.Sprintf(CLAB_HOSTENTRY_POSTFIX+"\n", labname))
	return []byte(entries.String())
}

func DeleteEntriesFromHostsFile(labname string) error {
	if labname == "" {
		return fmt.Errorf("missing containerlab name")
	}
	f, err := os.OpenFile("/etc/hosts", os.O_RDWR, 0644) // skipcq: GSC-G302
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Info("/etc/hosts does not exist")
		} else {
			return err
		}
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	skiplines := false
	output := bytes.Buffer{}
	prefix := fmt.Sprintf(CLAB_HOSTENTRY_PREFIX, labname)
	postfix := fmt.Sprintf(CLAB_HOSTENTRY_POSTFIX, labname)
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
		return fmt.Errorf("issue cleaning up /etc/hosts file. Please do so manually")
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
