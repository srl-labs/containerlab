// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	gover "github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

var (
	version = "0.0.0"
	commit  = "none"
	date    = "unknown"
)

const (
	repoUrl = "https://github.com/srl-labs/containerlab"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var slug = `
                           _                   _       _     
                 _        (_)                 | |     | |    
 ____ ___  ____ | |_  ____ _ ____   ____  ____| | ____| | _  
/ ___) _ \|  _ \|  _)/ _  | |  _ \ / _  )/ ___) |/ _  | || \ 
( (__| |_|| | | | |_( ( | | | | | ( (/ /| |   | ( ( | | |_) )
\____)___/|_| |_|\___)_||_|_|_| |_|\____)_|   |_|\_||_|____/ 
`

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show containerlab version or upgrade",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(slug)
		verSlug := docsLinkFromVer(version)
		fmt.Printf("    version: %s\n", version)
		fmt.Printf("     commit: %s\n", commit)
		fmt.Printf("       date: %s\n", date)
		fmt.Printf("     source: %s\n", repoUrl)
		fmt.Printf(" rel. notes: https://containerlab.dev/rn/%s\n", verSlug)
	},
}

// get LatestVersion fetches latest containerlab release version from Github releases.
func getLatestVersion(ctx context.Context, vc chan string) { // skipcq: RVV-A0006
	// client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD",
		fmt.Sprintf("%s/releases/latest", repoUrl), nil)
	if err != nil {
		log.Debugf("error occurred during latest version fetch: %v", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 302 {
		log.Debugf("error occurred during latest version fetch: %v", err)
		return
	}

	loc := resp.Header.Get("Location")
	split := strings.Split(loc, "releases/tag/")

	// latest version
	vL, _ := gover.NewVersion(split[1])
	// current version
	vC, _ := gover.NewVersion(version)

	if vL.GreaterThan(vC) {
		log.Debugf("latest version %s is newer than the current one %s\n", vL.String(), vC.String())
		vc <- vL.String()
	}

	resp.Body.Close()
}

// newVerNotification prints logs information about a new version if one was found.
func newVerNotification(vc chan string) {
	select {
	case ver, ok := <-vc:
		if ok {
			relSlug := docsLinkFromVer(ver)
			log.Infof("🎉 New containerlab version %s is available! Release notes: https://containerlab.dev/rn/%s\nRun 'containerlab version upgrade' to upgrade or go check other installation options at https://containerlab.dev/install/\n", ver, relSlug)
		}
	default:
		return
	}
}

// docsLinkFromVer creates a documentation path attribute for a given version
// for 0.15.0 version, the it returns 0.15/
// for 0.15.1 - 0.15/#0151.
func docsLinkFromVer(ver string) string {
	v, _ := gover.NewVersion(ver)
	segments := v.Segments()
	maj := segments[0]
	min := segments[1]
	patch := segments[2]

	relSlug := fmt.Sprintf("%d.%d/", maj, min)
	if patch != 0 {
		relSlug = relSlug + fmt.Sprintf("#%d%d%d", maj, min, patch)
	}
	return relSlug
}

// getLatestClabVersion returns a chan that returns the version check result
// uses the CLAB_VERSION_CHECK env variable (default true, if == "disable" will not perform the check).
func getLatestClabVersion(ctx context.Context) chan string {
	// latest version channel
	vCh := make(chan string)

	// check if new_version_notification is meant to be disabled
	versionCheckStatus := os.Getenv("CLAB_VERSION_CHECK")
	log.Debugf("Env: CLAB_VERSION_CHECK=%s", versionCheckStatus)

	if strings.Contains(strings.ToLower(versionCheckStatus), "disable") {
		close(vCh)
	} else {
		go getLatestVersion(ctx, vCh)
	}

	return vCh
}
