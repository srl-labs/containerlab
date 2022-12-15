// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var downloadURL = "https://github.com/srl-labs/containerlab/raw/main/get.sh"

// upgradeCmd represents the version command.
var upgradeCmd = &cobra.Command{
	Use:     "upgrade",
	Short:   "upgrade containerlab to latest available version",
	PreRunE: sudoCheck,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.CreateTemp("", "containerlab")
		defer os.Remove(f.Name())
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		_ = downloadFile(downloadURL, f)

		c := exec.Command("bash", f.Name())
		// c.Stdin = os.Stdin
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
func downloadFile(url string, file *os.File) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	versionCmd.AddCommand(upgradeCmd)
}
