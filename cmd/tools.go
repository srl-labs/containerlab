package cmd

import (
	"github.com/spf13/cobra"
)

// toolsCmd represents the tools command
var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "various tools your lab might need",
	Long:  "tools command groups various tools you might need for your lab\nreference: https://containerlab.srlinux.dev/cmd/tools/",
}

func init() {
	rootCmd.AddCommand(toolsCmd)
}
