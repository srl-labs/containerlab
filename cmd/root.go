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
	"github.com/srl-labs/containerlab/cmd/version"
	"github.com/srl-labs/containerlab/git"
	"github.com/srl-labs/containerlab/utils"
)

var (
	debugCount       int
	logLevel         string
	debug            bool
	timeout          time.Duration
	topoFile         string
	varsFile         string
	graph            bool
	runtime          string
	nodeFilter       []string
	labName          string
	gracefulShutdown bool
)

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:               "containerlab",
	Short:             "deploy container based lab environments with a user-defined interconnections",
	PersistentPreRunE: preRunFn,
	Aliases:           []string{"clab"},
}

func addSubcommands() {
	RootCmd.AddCommand(InspectCmd)
	RootCmd.AddCommand(version.VersionCmd)
}

func init() {
	RootCmd.SilenceUsage = true
	RootCmd.PersistentFlags().CountVarP(&debugCount, "debug", "d", "enable debug mode")
	RootCmd.PersistentFlags().StringVarP(&topoFile, "topo", "t", "",
		"path to the topology definition file, a directory containing one, 'stdin', or a URL")
	RootCmd.PersistentFlags().StringVarP(&varsFile, "vars", "", "",
		"path to the topology template variables file")
	_ = RootCmd.MarkPersistentFlagFilename("topo", "*.yaml", "*.yml")
	RootCmd.PersistentFlags().StringVarP(&labName, "name", "", "", "lab name")
	RootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "", 120*time.Second,
		"timeout for external API requests (e.g. container runtimes), e.g: 30s, 1m, 2m30s")
	RootCmd.PersistentFlags().StringVarP(&runtime, "runtime", "r", "", "container runtime")
	RootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", "info",
		"logging level; one of [trace, debug, info, warning, error, fatal]")

	addSubcommands()
}

func preRunFn(cobraCmd *cobra.Command, _ []string) error {
	// setting log level
	switch {
	case debugCount > 0:
		log.SetLevel(log.DebugLevel)
	default:
		l, err := log.ParseLevel(logLevel)
		if err != nil {
			return err
		}

		log.SetLevel(l)
	}

	// initializes the version manager that goes off and fetches current version in
	// the background for us
	version.InitManager(cobraCmd.Context())

	// setting output to stderr, so that json outputs can be parsed
	log.SetOutput(os.Stderr)

	log.SetTimeFormat(time.TimeOnly)

	err := utils.DropRootPrivs()
	if err != nil {
		return err
	}
	// Rootless operations only supported for Docker runtime
	if runtime != "" && runtime != "docker" {
		err := utils.CheckAndGetRootPrivs(cobraCmd, nil)
		if err != nil {
			return err
		}
	}

	return getTopoFilePath(cobraCmd)
}

// getTopoFilePath finds *.clab.y*ml file in the current working directory
// if the file was not specified.
// If the topology file refers to a git repository, it will be cloned to the current directory.
// Errors if more than one file is found by the glob path.
func getTopoFilePath(cmd *cobra.Command) error { // skipcq: GO-R1005
	// set commands which may use topo file find functionality, the rest don't need it
	if cmd.Name() != "deploy" &&
		cmd.Name() != "destroy" &&
		cmd.Name() != "redeploy" &&
		cmd.Name() != "inspect" &&
		cmd.Name() != "save" &&
		cmd.Name() != "graph" &&
		cmd.Name() != "interfaces" {
		return nil
	}

	// inspect and destroy commands with --all flag don't use file find functionality
	if (cmd.Name() == "inspect" || cmd.Name() == "destroy") &&
		cmd.Flag("all").Value.String() == "true" {
		return nil
	}

	var err error
	// perform topology clone/fetch if the topo file is not available locally
	if !utils.FileOrDirExists(topoFile) {
		switch {
		case git.IsGitHubOrGitLabURL(topoFile) || git.IsGitHubShortURL(topoFile):
			topoFile, err = processGitTopoFile(topoFile)
			if err != nil {
				return err
			}
		case utils.IsHttpURL(topoFile, true):
			// canonize the passed topo as URL by adding https schema if it was missing
			if !strings.HasPrefix(topoFile, "http://") &&
				!strings.HasPrefix(topoFile, "https://") {
				topoFile = "https://" + topoFile
			}
		}
	}

	// if topo or name flags have been provided, don't try to derive the topo file
	if topoFile != "" || labName != "" {
		return nil
	}

	log.Debugf("trying to find topology files automatically")

	files, err := filepath.Glob("*.clab.y*ml")

	if len(files) == 0 {
		return errors.New("no topology files matching the pattern *.clab.yml or *.clab.yaml found")
	}

	if len(files) > 1 {
		return fmt.Errorf("more than one topology file matching the pattern *.clab.yml or *.clab.yaml found, can't pick one: %q", files)
	}

	topoFile = files[0]

	log.Debugf("topology file found: %s", files[0])

	return err
}

func processGitTopoFile(topo string) (string, error) {
	// for short github urls, prepend https://github.com
	// note that short notation only works for github links
	if git.IsGitHubShortURL(topo) {
		topo = "https://github.com/" + topo
	}

	repo, err := git.NewRepo(topo)
	if err != nil {
		return "", err
	}

	// Instantiate the git implementation to use.
	gitImpl := git.NewGoGit(repo)

	// clone the repo via the Git Implementation
	err = gitImpl.Clone()
	if err != nil {
		return "", err
	}

	// adjust permissions for the checked out repo
	// it would belong to root/root otherwise
	err = utils.SetUIDAndGID(repo.GetName())
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
