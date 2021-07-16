package clab

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

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
	buff := bytes.Buffer{}
	v4buff := bytes.Buffer{}
	v6buff := bytes.Buffer{}

	for _, cont := range containers {
		if len(cont.Names) == 0 {
			continue
		}
		if cont.NetworkSettings.Set {
			if cont.NetworkSettings.IPv4addr != "" {
				v4buff.WriteString(cont.NetworkSettings.IPv4addr)
				v4buff.WriteString("\t")
				v4buff.WriteString(strings.TrimLeft(cont.Names[0], "/"))
				v4buff.WriteString("\n")
			}
			if cont.NetworkSettings.IPv6addr != "" {
				v6buff.WriteString(cont.NetworkSettings.IPv6addr)
				v6buff.WriteString("\t")
				v6buff.WriteString(strings.TrimLeft(cont.Names[0], "/"))
				v6buff.WriteString("\n")
			}
		}
	}
	// combine stuff
	buff.WriteString(fmt.Sprintf(CLAB_HOSTENTRY_PREFIX+"\n", labname))
	buff.WriteString(v4buff.String())
	buff.WriteString(v6buff.String())
	buff.WriteString(fmt.Sprintf(CLAB_HOSTENTRY_POSTFIX+"\n", labname))
	return buff.Bytes()
}

func DeleteEntriesFromHostsFile(labname string) error {
	if labname == "" {
		return fmt.Errorf("missing containerlab name")
	}
	f, err := os.OpenFile("/etc/hosts", os.O_RDWR, 0644) // skipcq: GSC-G302
	if err != nil {
		return err
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
