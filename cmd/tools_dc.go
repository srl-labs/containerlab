// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
)

// shellDefaults maps image substrings (checked in order) to the shell argv to use.
var shellDefaults = []struct {
	imageKey string
	argv     []string
}{
	{"ceos", []string{"/usr/bin/Cli", "-p", "15"}},
	{"nokia/srlinux", []string{"/opt/srlinux/bin/sr_cli"}},
	{"ipng/vpp-containerlab", []string{"/usr/bin/nsenter", "--net=/run/netns/dataplane", "/bin/bash"}},
	{"network-multitool", []string{"/bin/bash"}},
}

func shellForImage(image string) []string {
	for _, s := range shellDefaults {
		if strings.Contains(image, s.imageKey) {
			return s.argv
		}
	}

	return []string{"/bin/sh"}
}

func dockerConnectCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "dc [containername]",
		Short: "connect to a lab container using the appropriate shell",
		Long: "docker-connect: exec into a running containerlab container with the right shell.\n" +
			"When no container name is given, lists all running lab containers.\n" +
			"reference: https://containerlab.dev/cmd/tools/dc/",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return dockerConnect(cobraCmd, o, args)
		},
	}

	c.Flags().StringVarP(
		&o.ToolsDC.Shell,
		"shell",
		"s",
		o.ToolsDC.Shell,
		"shell command to use (overrides auto-detection based on image)",
	)

	return c, nil
}

func dockerConnect(cobraCmd *cobra.Command, o *Options, args []string) error {
	ctx := cobraCmd.Context()

	// Auto-discover a topology file in the current directory when the user
	// has not explicitly passed --topo or --name. This scopes the container
	// list to the local lab, avoiding noise on machines with many running labs.
	if o.Global.TopologyFile == "" && o.Global.TopologyName == "" {
		if found, err := clabcore.FindTopoFileByPath("."); err == nil {
			o.Global.TopologyFile = found
		}
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	containers, err := listContainers(ctx, c, o)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		if len(containers) == 0 {
			fmt.Println("no running lab containers found")
			return nil
		}

		fmt.Println("available containers:")

		for _, ct := range containers {
			name := strings.TrimPrefix(ct.Names[0], "/")
			fmt.Printf("  %s (%s)\n", name, ct.Image)
		}

		return nil
	}

	filter := args[0]

	var matches []struct {
		name  string
		image string
	}

	for _, ct := range containers {
		name := strings.TrimPrefix(ct.Names[0], "/")
		if strings.Contains(name, filter) {
			matches = append(matches, struct {
				name  string
				image string
			}{name, ct.Image})
		}
	}

	switch len(matches) {
	case 0:
		fmt.Fprintln(os.Stderr, "no match found. available containers:")

		for _, ct := range containers {
			name := strings.TrimPrefix(ct.Names[0], "/")
			fmt.Fprintf(os.Stderr, "  %s (%s)\n", name, ct.Image)
		}

		return fmt.Errorf("no container matching %q", filter)
	case 1:
		// exactly one match — proceed
	default:
		fmt.Fprintln(os.Stderr, "ambiguous argument, matched more than one container:")

		for _, m := range matches {
			fmt.Fprintf(os.Stderr, "  %s (%s)\n", m.name, m.image)
		}

		return fmt.Errorf("ambiguous container name %q", filter)
	}

	match := matches[0]

	var shell []string
	if o.ToolsDC.Shell != "" {
		shell = strings.Fields(o.ToolsDC.Shell)
	} else {
		shell = shellForImage(match.image)
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker executable not found in PATH: %w", err)
	}

	argv := append([]string{"docker", "exec", "-it", match.name}, shell...)

	return syscall.Exec(dockerPath, argv, os.Environ())
}
