// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func completionCmd(_ *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "generate completion script",
		Long: `To load completions:

Bash:

  $ source <(containerlab completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ containerlab completion bash > /etc/bash_completion.d/containerlab
  # macOS:
  $ containerlab completion bash > /usr/local/etc/bash_completion.d/containerlab

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ containerlab completion zsh > "${fpath[1]}/_containerlab"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ containerlab completion fish | source

  # To load completions for each session, execute once:
  $ containerlab completion fish > ~/.config/fish/completions/containerlab.fish
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				_ = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				_ = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				_ = cmd.Root().GenFishCompletion(os.Stdout, true)
			}
		},
	}

	return c, nil
}
