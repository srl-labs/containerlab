// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabgit "github.com/srl-labs/containerlab/git"
	clabruntimedocker "github.com/srl-labs/containerlab/runtime/docker"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func subcommandRegisterFuncs() []func(*Options) (*cobra.Command, error) {
	return []func(*Options) (*cobra.Command, error){
		versionCmd,
		completionCmd,
		configCmd,
		deployCmd,
		destroyCmd,
		execCmd,
		generateCmd,
		graphCmd,
		inspectCmd,
		redeployCmd,
		saveCmd,
		toolsCmd,
	}
}

// Entrypoint returns the root cobra command to be executed or errors if it cannot do so.
func Entrypoint() (*cobra.Command, error) {
	o := GetOptions()

	c := &cobra.Command{
		Use:   "containerlab",
		Short: "deploy container based lab environments with a user-defined interconnections",
		PersistentPreRunE: func(cobraCmd *cobra.Command, _ []string) error {
			return preRunFn(cobraCmd, o)
		},
		Aliases:      []string{"clab"},
		SilenceUsage: true,
	}

	c.PersistentFlags().CountVarP(
		&o.Global.DebugCount,
		"debug",
		"d",
		"enable debug mode",
	)
	c.PersistentFlags().StringVarP(
		&o.Global.TopologyFile,
		"topo",
		"t",
		o.Global.TopologyFile,
		"path to the topology definition file, a directory containing one, 'stdin', or a URL",
	)
	c.PersistentFlags().StringVarP(
		&o.Global.VarsFile,
		"vars",
		"",
		o.Global.VarsFile,
		"path to the topology template variables file",
	)
	c.PersistentFlags().StringVarP(
		&o.Global.TopologyName,
		"name",
		"",
		o.Global.TopologyName,
		"lab/topology name")
	c.PersistentFlags().DurationVarP(
		&o.Global.Timeout,
		"timeout",
		"",
		o.Global.Timeout,
		"timeout for external API requests (e.g. container runtimes), e.g: 30s, 1m, 2m30s",
	)
	c.PersistentFlags().StringVarP(
		&o.Global.Runtime,
		"runtime",
		"r",
		o.Global.Runtime,
		"container runtime",
	)
	c.PersistentFlags().StringVarP(
		&o.Global.LogLevel,
		"log-level",
		"",
		o.Global.LogLevel,
		"logging level; one of [trace, debug, info, warning, error, fatal]",
	)

	err := c.MarkPersistentFlagFilename("topo", "*.yaml", "*.yml")
	if err != nil {
		return nil, err
	}

	for _, f := range subcommandRegisterFuncs() {
		cmd, err := f(o)
		if err != nil {
			return nil, err
		}

		c.AddCommand(cmd)
	}

	return c, nil
}

func preRunFn(cobraCmd *cobra.Command, o *Options) error {
	// setting log level
	switch {
	case o.Global.DebugCount > 0:
		log.SetLevel(log.DebugLevel)
	default:
		l, err := log.ParseLevel(o.Global.LogLevel)
		if err != nil {
			return err
		}

		log.SetLevel(l)
	}

	// initializes the version manager that goes off and fetches current version in
	// the background for us
	initVersionManager(cobraCmd.Context())

	// setting output to stderr, so that json outputs can be parsed
	log.SetOutput(os.Stderr)

	log.SetTimeFormat(time.TimeOnly)

	err := clabutils.DropRootPrivs()
	if err != nil {
		return err
	}
	// Rootless operations only supported for Docker runtime
	if o.Global.Runtime != "" && o.Global.Runtime != clabruntimedocker.RuntimeName {
		err := clabutils.CheckAndGetRootPrivs()
		if err != nil {
			return err
		}
	}

	return getTopoFilePath(cobraCmd, o)
}

// getTopoFilePath finds *.clab.y*ml file in the current working directory
// if the file was not specified.
// If the topology file refers to a git repository, it will be cloned to the current directory.
// Errors if more than one file is found by the glob path.
func getTopoFilePath(cobraCmd *cobra.Command, o *Options) error { // skipcq: GO-R1005
	// set commands which may use topo file find functionality, the rest don't need it
	if cobraCmd.Name() != "deploy" &&
		cobraCmd.Name() != "destroy" &&
		cobraCmd.Name() != "redeploy" &&
		cobraCmd.Name() != "inspect" &&
		cobraCmd.Name() != "save" &&
		cobraCmd.Name() != "graph" &&
		cobraCmd.Name() != "interfaces" {
		return nil
	}

	// inspect and destroy commands with --all flag don't use file find functionality
	if (cobraCmd.Name() == "inspect" || cobraCmd.Name() == "destroy") &&
		cobraCmd.Flag("all").Value.String() == "true" {
		return nil
	}

	var err error
	// perform topology clone/fetch if the topo file is not available locally
	if !clabutils.FileOrDirExists(o.Global.TopologyFile) {
		switch {
		case clabgit.IsGitHubOrGitLabURL(o.Global.TopologyFile) ||
			clabgit.IsGitHubShortURL(o.Global.TopologyFile):
			o.Global.TopologyFile, err = processGitTopoFile(o.Global.TopologyFile)
			if err != nil {
				return err
			}
		case clabutils.IsHttpURL(o.Global.TopologyFile, true):
			// canonize the passed topo as URL by adding https schema if it was missing
			if !strings.HasPrefix(o.Global.TopologyFile, "http://") &&
				!strings.HasPrefix(o.Global.TopologyFile, "https://") {
				o.Global.TopologyFile = "https://" + o.Global.TopologyFile
			}
		}
	}

	// if topo or name flags have been provided, don't try to derive the topo file
	if o.Global.TopologyFile != "" || o.Global.TopologyName != "" {
		return nil
	}

	log.Debugf("trying to find topology files automatically")

	files, err := filepath.Glob("*.clab.y*ml")

	if len(files) == 0 {
		return errors.New("no topology files matching the pattern *.clab.yml or *.clab.yaml found")
	}

	if len(files) > 1 {
		return fmt.Errorf(
			"more than one topology file matching the pattern *.clab.yml or *.clab.yaml found, "+
				"can't pick one: %q",
			files,
		)
	}

	o.Global.TopologyFile = files[0]

	log.Debugf("topology file found: %s", files[0])

	return err
}

func processGitTopoFile(topo string) (string, error) {
	// for short github urls, prepend https://github.com
	// note that short notation only works for github links
	if clabgit.IsGitHubShortURL(topo) {
		topo = "https://github.com/" + topo
	}

	repo, err := clabgit.NewRepo(topo)
	if err != nil {
		return "", err
	}

	// Instantiate the git implementation to use.
	gitImpl := clabgit.NewGoGit(repo)

	// clone the repo via the Git Implementation
	err = gitImpl.Clone()
	if err != nil {
		return "", err
	}

	// adjust permissions for the checked out repo
	// it would belong to root/root otherwise
	err = clabutils.SetUIDAndGID(repo.GetName())
	if err != nil {
		log.Errorf("error adjusting repository permissions %v. Continuing anyways", err)
	}

	// prepare the path with the repo based path
	path := filepath.Join(repo.GetPath()...)
	// prepend that path with the repo base directory
	path = filepath.Join(repo.GetName(), path)

	// change dir to the
	err = os.Chdir(path)
	if err != nil {
		return "", err
	}

	return repo.GetFilename(), err
}
