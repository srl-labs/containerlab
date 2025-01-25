// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package version

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	gover "github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
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
//go:embed logo.txt
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
	maj := segments[0]
	min := segments[1]
	patch := segments[2]

	relSlug := fmt.Sprintf("%d.%d/", maj, min)
	if patch != 0 {
		relSlug += fmt.Sprintf("#%d%d%d", maj, min, patch)
	}
	return relSlug
}

// GetLatestClabVersion optional function for a background check. It respects
// CLAB_VERSION_CHECK="disable" to skip remote calls. Typically used in your
// "deploy" or other commands if you want a background version check.
func GetLatestClabVersion(ctx context.Context) chan string {
	vCh := make(chan string, 1)

	// check if version check is disabled
	versionCheckStatus := os.Getenv("CLAB_VERSION_CHECK")
	log.Debugf("Env: CLAB_VERSION_CHECK=%s", versionCheckStatus)

	if strings.Contains(strings.ToLower(versionCheckStatus), "disable") {
		close(vCh)
		return vCh
	}

	go getLatestVersion(ctx, vCh)
	return vCh
}
