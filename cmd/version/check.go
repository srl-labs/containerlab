package version

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// checkCmd defines a version check command.
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if a new version of containerlab is available",
	RunE: func(cobraCmd *cobra.Command, _ []string) error {
		// We'll use a short 5-second timeout for the remote request
		ctx, cancel := context.WithTimeout(cobraCmd.Context(), 5*time.Second)
		defer cancel()

		m := GetManager()
		m.DisplayNewVersionAvailable(ctx)

		return nil
	},
}

// printNewVersionInfo prints instructions about a
// newer version, so we don't duplicate that string in multiple places.
func printNewVersionInfo(ver string) {
	relSlug := docsLinkFromVer(ver)
	fmt.Printf("🎉 A newer containerlab version (%s) is available!\n", ver)
	fmt.Printf("Release notes: https://containerlab.dev/rn/%s\n", relSlug)
	fmt.Println("Run 'sudo clab version upgrade' or see https://containerlab.dev/install/ for installation options.")
}
