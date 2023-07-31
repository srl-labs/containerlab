// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	debugCount int
	debug      bool
	timeout    time.Duration
	logLevel   string
)

// path to the topology file.
var topo string

var (
	varsFile string
	graph    bool
	rt       string
)

// lab name.
var name string

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:               "containerlab",
	Short:             "deploy container based lab environments with a user-defined interconnections",
	PersistentPreRunE: preRunFn,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1) // skipcq: RVV-A0003
	}
}

func init() {
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().CountVarP(&debugCount, "debug", "d", "enable debug mode")
	rootCmd.PersistentFlags().StringVarP(&topo, "topo", "t", "", "path to the topology file")
	rootCmd.PersistentFlags().StringVarP(&varsFile, "vars", "", "",
		"path to the topology template variables file")
	_ = rootCmd.MarkPersistentFlagFilename("topo", "*.yaml", "*.yml")
	rootCmd.PersistentFlags().StringVarP(&name, "name", "", "", "lab name")
	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "", 120*time.Second,
		"timeout for external API requests (e.g. container runtimes), e.g: 30s, 1m, 2m30s")
	rootCmd.PersistentFlags().StringVarP(&rt, "runtime", "r", "", "container runtime")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", "info",
		"logging level; one of [trace, debug, info, warning, error, fatal]")
}

func sudoCheck(_ *cobra.Command, _ []string) error {
	id := os.Geteuid()
	if id != 0 {
		return errors.New("containerlab requires sudo privileges to run")
	}
	return nil
}

func preRunFn(cmd *cobra.Command, _ []string) error {
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

	// setting output to stderr, so that json outputs can be parsed
	log.SetOutput(os.Stderr)

	return getTopoFilePath(cmd)
}

// getTopoFilePath finds *.clab.y*ml file in the current working directory if the files was note specified using flags
// errors if more than one file is found by the glob path.
func getTopoFilePath(cmd *cobra.Command) error {
	// set commands which may use topo file find functionality, the rest don't need it
	if !(cmd.Name() == "deploy" || cmd.Name() == "destroy" || cmd.Name() == "inspect" ||
		cmd.Name() == "save" || cmd.Name() == "graph" || cmd.Name() == "exec") {
		return nil
	}

	// inspect and destroy commands with --all flag don't use file find functionality
	if (cmd.Name() == "inspect" || cmd.Name() == "destroy") &&
		cmd.Flag("all").Value.String() == "true" {
		return nil
	}

	// if topo or name flags have been provided, don't try to derive the topo file
	if topo != "" || name != "" {
		return nil
	}

	var err error

	log.Debugf("trying to find topology files automatically")

	files, err := filepath.Glob("*.clab.y*ml")

	if len(files) != 1 {
		return errors.New("none or more than one topology files found, can't auto select one")
	}

	topo = files[0]

	log.Debugf("topology file found: %s", files[0])

	return err
}
