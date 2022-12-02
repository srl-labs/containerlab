// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	terminal "golang.org/x/term"
)

const loginEndpoint = "https://api.mysocket.io/login"

var (
	email    string
	password string
)

type tokenResp struct {
	Token string
}

func init() {
	toolsCmd.AddCommand(mysocketioCmd)

	mysocketioCmd.AddCommand(mysocketioLoginCmd)
	mysocketioLoginCmd.Flags().StringVarP(&email, "email", "e", "", "Email address")
	mysocketioLoginCmd.Flags().StringVarP(&password, "password", "p", "", "Password")
	_ = mysocketioLoginCmd.MarkFlagRequired("email")
}

// vxlanCmd represents the vxlan command container.
var mysocketioCmd = &cobra.Command{
	Use:   "mysocketio",
	Short: "Mysocket.io commands",
}

// mysocketioLoginCmd represents the mysocketio-login command.
var mysocketioLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Logs in to mysocket.io service and saves a token to current working directory",

	RunE: func(cmd *cobra.Command, args []string) error {
		if password == "" {
			var err error
			password, err = readPassword()
			if err != nil {
				return err
			}
		}
		reqBody, err := json.Marshal(map[string]string{
			"email":    email,
			"password": password,
		})
		if err != nil {
			return err
		}
		resp, err := http.Post(loginEndpoint, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var t tokenResp
		err = json.Unmarshal(b, &t)
		if err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		err = os.WriteFile(filepath.Join(cwd, ".mysocketio_token"), []byte(t.Token), 0640)
		if err != nil {
			return fmt.Errorf("failed to write mysocketio token file as %s: %v",
				filepath.Join(cwd, ".mysocketio_token"), err)
		}
		log.Infof("Saved mysocketio token as %s", filepath.Join(cwd, ".mysocketio_token"))
		return nil
	},
}

func readPassword() (string, error) {
	fmt.Print("password: ")
	pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(pass), nil
}
