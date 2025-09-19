// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	gover "github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// Version variables set at build time (e.g., with -ldflags).
var (
	Version = "0.0.0"
	commit  = "none"
	date    = "unknown"
)

const (
	repoUrl             = "https://github.com/srl-labs/containerlab"
	downloadURL         = "https://github.com/srl-labs/containerlab/raw/main/get.sh"
	versionCheckTimeout = 5 * time.Second
)

func versionCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "version",
		Short: "Show containerlab version or upgrade",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return printVersionInfoFn(cobraCmd, o)
		},
	}

	c.Flags().BoolVarP(&o.Version.Short, "short", "s", false, "Print just the version number")
	c.Flags().BoolVarP(&o.Version.JSON, "json", "j", false, "Print version info as json")

	c.AddCommand(
		&cobra.Command{
			Use:   "check",
			Short: "Check if a new version of containerlab is available",
			RunE: func(cobraCmd *cobra.Command, _ []string) error {
				// We'll use a short 5-second timeout for the remote request
				ctx, cancel := context.WithTimeout(cobraCmd.Context(), versionCheckTimeout)
				defer cancel()

				m := getVersionManager()
				m.DisplayNewVersionAvailable(ctx)

				return nil
			},
		},
		&cobra.Command{
			Use:   "upgrade",
			Short: "upgrade containerlab to latest available version",
			PreRunE: func(_ *cobra.Command, _ []string) error {
				return clabutils.CheckAndGetRootPrivs()
			},
			RunE: upgrade,
		},
	)

	return c, nil
}

// this a note to self how color codes work
// https://stackoverflow.com/questions/4842424/list-of-ansi-color-escape-sequences
// https://patorjk.com/software/taag/#p=display&f=Ivrit&t=CONTAINERlab
//
//go:embed assets/logo.txt
var projASCIILogo string

// docsLinkFromVer: creates a documentation path for a given version
// e.g., for 0.15.0 => 0.15/
//
// for 0.15.1 => 0.15/#0151.
func docsLinkFromVer(ver string) string {
	v, err := gover.NewVersion(ver)
	if err != nil {
		return "" // fallback
	}

	segments := v.Segments()
	major := segments[0]
	minor := segments[1]
	patch := segments[2]

	relSlug := fmt.Sprintf("%d.%d/", major, minor)
	if patch != 0 {
		relSlug += fmt.Sprintf("#%d%d%d", major, minor, patch)
	}

	return relSlug
}

func printVersionInfoFn(_ *cobra.Command, o *Options) error {
	versionData := struct {
		Version      string `json:"version"`
		Commit       string `json:"commit"`
		Date         string `json:"date"`
		RepoUrl      string `json:"repository"`
		ReleaseNotes string `json:"releaseNotes"`
	}{
		Version:      Version,
		Commit:       commit,
		Date:         date,
		RepoUrl:      repoUrl,
		ReleaseNotes: fmt.Sprintf("https://containerlab.dev/rn/%s", docsLinkFromVer(Version)),
	}

	if o.Version.Short {
		fmt.Println(Version)
		return nil
	}

	if o.Version.JSON {
		j, err := json.Marshal(versionData)
		if err != nil {
			return err
		}

		fmt.Println(string(j))

		return nil
	}

	fmt.Println(projASCIILogo)
	fmt.Printf("    version: %s\n", Version)
	fmt.Printf("     commit: %s\n", commit)
	fmt.Printf("       date: %s\n", date)
	fmt.Printf("     source: %s\n", repoUrl)
	fmt.Printf(" rel. notes: %s\n", versionData.ReleaseNotes)

	return nil
}

// printNewVersionInfo prints instructions about a
// newer version, so we don't duplicate that string in multiple places.
func printNewVersionInfo(ver string) {
	relSlug := docsLinkFromVer(ver)
	fmt.Printf("🎉 A newer containerlab version (%s) is available!\n", ver)
	fmt.Printf("Release notes: https://containerlab.dev/rn/%s\n", relSlug)
	fmt.Println(
		"Run 'sudo clab version upgrade' or see https://containerlab.dev/install/ " +
			"for installation options.",
	)
}

func upgrade(cobraCmd *cobra.Command, _ []string) error {
	f, err := os.CreateTemp("", "containerlab")

	defer os.Remove(f.Name())

	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	err = clabutils.CopyFileContents(cobraCmd.Context(), downloadURL, f)
	if err != nil {
		return fmt.Errorf("failed to download upgrade script: %w", err)
	}

	c := exec.Command("sudo", "-E", "bash", f.Name())
	// pass the environment variables to the upgrade script
	// so that GITHUB_TOKEN is available
	c.Env = os.Environ()

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err = c.Run()
	if err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	return nil
}
