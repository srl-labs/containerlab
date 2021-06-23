// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var downloadURL = "https://github.com/srl-labs/containerlab/raw/master/get.sh"

// upgradeCmd represents the version command
var upgradeCmd = &cobra.Command{
	Use:     "upgrade",
	Short:   "upgrade containerlab to latest available version",
	PreRunE: sudoCheck,
	Run: func(cmd *cobra.Command, args []string) {
		f, err := ioutil.TempFile("", "containerlab")
		defer os.Remove(f.Name())
		if err != nil {
			log.Fatalf("Failed to create temp file %s\n", err)
		}
		_ = downloadFile(downloadURL, f)

		c := exec.Command("bash", f.Name())
		// c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		err = c.Run()
		if err != nil {
			log.Fatalf("Upgrade failed: %s\n", err)
		}
	},
}

// downloadFile will download a file from a URL and write its content to a file
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
