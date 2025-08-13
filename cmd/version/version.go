// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package version

import (
	_ "embed"
	"fmt"

	gover "github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
)

// Version variables set at build time (e.g., with -ldflags).
var (
	Version = "0.0.0"
	commit  = "none"
	date    = "unknown"
)

const repoUrl = "https://github.com/srl-labs/containerlab"

func init() {
	// Add "check" subcommand under "version"
	VersionCmd.AddCommand(checkCmd)
}

// this a note to self how color codes work
// https://stackoverflow.com/questions/4842424/list-of-ansi-color-escape-sequences
// https://patorjk.com/software/taag/#p=display&f=Ivrit&t=CONTAINERlab
//
//go:embed assets/logo.txt
var projASCIILogo string

// VersionCmd defines the version command.
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show containerlab version or upgrade",
	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Println(projASCIILogo)
		verSlug := docsLinkFromVer(Version)
		fmt.Printf("    version: %s\n", Version)
		fmt.Printf("     commit: %s\n", commit)
		fmt.Printf("       date: %s\n", date)
		fmt.Printf("     source: %s\n", repoUrl)
		fmt.Printf(" rel. notes: https://containerlab.dev/rn/%s\n", verSlug)
		return nil
	},
}

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
