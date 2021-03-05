package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
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

PowerShell:

  PS> containerlab completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> containerlab completion powershell > containerlab.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
