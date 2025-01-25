package version

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	gover "github.com/hashicorp/go-version"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// checkCmd defines a version check command.
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

// getLatestVersion fetches the latest containerlab release version from GitHub
// and sends that version string to the vc channel if it's newer than local.
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
	vC, err := gover.NewVersion(Version)
	if err != nil {
		return
	}

	if vL.GreaterThan(vC) {
		log.Debugf("Latest version %s is newer than current %s\n", vL.String(), vC.String())
		vc <- vL.String()
	}
}

// NewVerNotification: non-blocking check that prints an INFO log if a new
// version is available. Useful for "background" checks in long-running commands
// (like "deploy") where we don't want to block the user.
func NewVerNotification(vc <-chan string) {
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

// newVerNotificationWithTimeout: a blocking check that waits for version info
// or a context timeout. Suitable for subcommands like "check" that want to wait
// for a definite result or a quick fallback if unreachable.
func newVerNotificationWithTimeout(ctx context.Context, vc <-chan string) {
	select {
	case ver := <-vc:
		if ver == "" {
			fmt.Printf("You are on the latest version (%s)\n", Version)
		} else {
			printNewVersionInfo(ver)
		}
	case <-ctx.Done():
		fmt.Println("Version check timed out or encountered an error.")
	}
}

// printNewVersionInfo prints instructions about a
// newer version, so we don't duplicate that string in multiple places.
func printNewVersionInfo(ver string) {
	relSlug := docsLinkFromVer(ver)
	fmt.Printf("🎉 A newer containerlab version (%s) is available!\n", ver)
	fmt.Printf("Release notes: https://containerlab.dev/rn/%s\n", relSlug)
	fmt.Println("Run 'sudo clab version upgrade' or see https://containerlab.dev/install/ for installation options.")
}
