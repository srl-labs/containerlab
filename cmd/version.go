package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "show containerlab version",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("version : %s\n", version)
		fmt.Printf(" commit : %s\n", commit)
		fmt.Printf("   date : %s\n", date)
		fmt.Printf(" source : https://github.com/srl-wim/container-lab\n")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
