// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
    _ "embed"
    "context"
    "fmt"
    "net/http"
    "os"
    "strings"
    "time"

    gover "github.com/hashicorp/go-version"
    log "github.com/sirupsen/logrus"
    "github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Version variables set at build time (e.g., with -ldflags)
// -----------------------------------------------------------------------------
var (
    version = "0.0.0"
    commit  = "none"
    date    = "unknown"
)

const repoUrl = "https://github.com/srl-labs/containerlab"

func init() {
    // Add "version" to root command
    rootCmd.AddCommand(versionCmd)
    // Add "check" subcommand under "version"
    versionCmd.AddCommand(checkCmd)
}

// this a note to self how color codes work
// https://stackoverflow.com/questions/4842424/list-of-ansi-color-escape-sequences
// https://patorjk.com/software/taag/#p=display&f=Ivrit&t=CONTAINERlab
//
//go:embed logo.txt
var projASCIILogo string

// -----------------------------------------------------------------------------
// versionCmd: prints local version info
// -----------------------------------------------------------------------------
var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Show containerlab version or upgrade",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println(projASCIILogo)
        verSlug := docsLinkFromVer(version)
        fmt.Printf("    version: %s\n", version)
        fmt.Printf("     commit: %s\n", commit)
        fmt.Printf("       date: %s\n", date)
        fmt.Printf("     source: %s\n", repoUrl)
        fmt.Printf(" rel. notes: https://containerlab.dev/rn/%s\n", verSlug)
        return nil
    },
}

// -----------------------------------------------------------------------------
// checkCmd: runs a blocking version check with a short timeout
// -----------------------------------------------------------------------------
var checkCmd = &cobra.Command{
    Use:   "check",
    Short: "Check if a new version of containerlab is available",
    RunE: func(cmd *cobra.Command, args []string) error {
        // We'll use a short 5-second timeout for the remote request
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        // Version check function returns a channel
        vCh := make(chan string, 1) // buffered to avoid potential goroutine leak

        // perform the version check in the background
        go getLatestVersion(ctx, vCh)

        // We want a *blocking* approach (we'll wait on vCh or a timeout)
        // so let's call our "blocking" notification helper:
        newVerNotificationWithTimeout(ctx, vCh)

        return nil
    },
}

// -----------------------------------------------------------------------------
// getLatestVersion fetches the latest containerlab release version from GitHub
// and sends that version string to the vc channel if it's newer than local
// -----------------------------------------------------------------------------
func getLatestVersion(ctx context.Context, vc chan<- string) {
    defer close(vc)

    client := &http.Client{
        // Don’t follow redirects
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }

    req, err := http.NewRequestWithContext(ctx, "HEAD", fmt.Sprintf("%s/releases/latest", repoUrl), nil)
    if err != nil {
        log.Debugf("error occurred during latest version fetch: %v", err)
        return
    }

    resp, err := client.Do(req)
    if err != nil || resp == nil || resp.StatusCode != 302 {
        if err == nil {
            err = fmt.Errorf("unexpected status code %d", resp.StatusCode)
        }
        log.Debugf("error occurred during latest version fetch: %v", err)
        return
    }
    defer resp.Body.Close()

    loc := resp.Header.Get("Location")
    split := strings.Split(loc, "releases/tag/")
    if len(split) != 2 {
        // can't parse version from redirect
        return
    }

    vL, err := gover.NewVersion(split[1])
    if err != nil {
        return
    }

    // parse current version
    vC, err := gover.NewVersion(version)
    if err != nil {
        return
    }

    if vL.GreaterThan(vC) {
        log.Debugf("Latest version %s is newer than current %s\n", vL.String(), vC.String())
        vc <- vL.String()
    }
}

// -----------------------------------------------------------------------------
// newVerNotification: non-blocking check that prints an INFO log if a new
// version is available. Useful for "background" checks in long-running commands
// (like "deploy") where we don't want to block the user.
// -----------------------------------------------------------------------------
func newVerNotification(vc <-chan string) {
    select {
    case ver, ok := <-vc:
        if ok && ver != "" {
            printNewVersionInfo(ver)
        }
    default:
        // no new version found or channel not ready
        return
    }
}

// -----------------------------------------------------------------------------
// newVerNotificationWithTimeout: a blocking check that waits for version info
// or a context timeout. Suitable for subcommands like "check" that want to wait
// for a definite result or a quick fallback if unreachable.
// -----------------------------------------------------------------------------
func newVerNotificationWithTimeout(ctx context.Context, vc <-chan string) {
    select {
    case ver := <-vc:
        if ver == "" {
            fmt.Printf("You are on the latest version (%s)\n", version)
        } else {
            printNewVersionInfo(ver)
        }
    case <-ctx.Done():
        fmt.Println("Version check timed out or encountered an error.")
    }
}

// -----------------------------------------------------------------------------
// printNewVersionInfo: a single function that prints instructions about a
// newer version, so we don't duplicate that string in multiple places
// -----------------------------------------------------------------------------
func printNewVersionInfo(ver string) {
    relSlug := docsLinkFromVer(ver)
    fmt.Printf("🎉 A newer containerlab version (%s) is available!\n", ver)
    fmt.Printf("Release notes: https://containerlab.dev/rn/%s\n", relSlug)
    fmt.Println("Run 'containerlab version upgrade' or see https://containerlab.dev/install/ for installation options.")
}

// -----------------------------------------------------------------------------
// docsLinkFromVer: creates a documentation path for a given version
// e.g., for 0.15.0 => 0.15/
//        for 0.15.1 => 0.15/#0151
// -----------------------------------------------------------------------------
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

// -----------------------------------------------------------------------------
// getLatestClabVersion: optional function for a background check. It respects
// CLAB_VERSION_CHECK="disable" to skip remote calls. Typically used in your
// "deploy" or other commands if you want a background version check.
// -----------------------------------------------------------------------------
func getLatestClabVersion(ctx context.Context) chan string {
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
