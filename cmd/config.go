package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var infra bool
var workload bool

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "generate and apply configuration to the lab",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("config called")
	},
}

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generate configuration for the lab",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("generate called")
	},
}

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "apply the generated configuration on the lab nodes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("apply called")
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(generateCmd)
	configCmd.AddCommand(applyCmd)
	//
	generateCmd.Flags().BoolVarP(&infra, "infra", "", false, "generate infra config")
	generateCmd.Flags().BoolVarP(&workload, "workload", "", false, "generate workloads config")
	//
	applyCmd.Flags().BoolVarP(&infra, "infra", "", false, "generate infra config")
	applyCmd.Flags().BoolVarP(&workload, "workload", "", false, "generate workloads config")
}
