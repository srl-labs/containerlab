// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package version

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/utils"
)

const downloadURL = "https://github.com/srl-labs/containerlab/raw/main/get.sh"

// upgradeCmd represents the version command.
var upgradeCmd = &cobra.Command{
	Use:     "upgrade",
	Short:   "upgrade containerlab to latest available version",
	PreRunE: utils.CheckAndGetRootPrivs,
	RunE: func(cobraCmd *cobra.Command, _ []string) error {
		f, err := os.CreateTemp("", "containerlab")
		defer os.Remove(f.Name())
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		err = downloadFile(cobraCmd.Context(), downloadURL, f)
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
	},
}

// downloadFile will download a file from a URL and write its content to a file.
func downloadFile(ctx context.Context, url string, file *os.File) error {
	// Create an HTTP client with specific transport
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	// Get the data
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close() // skipcq: GO-S2307

	// Write the body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	VersionCmd.AddCommand(upgradeCmd)
}
