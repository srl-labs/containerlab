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
var configGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generate configuration for the lab",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("generate called")
	},
}

// applyCmd represents the apply command
var configApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "apply the generated configuration on the lab nodes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("apply called")
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGenerateCmd)
	configCmd.AddCommand(configApplyCmd)
	//
	configGenerateCmd.Flags().BoolVarP(&infra, "infra", "", false, "generate infra config")
	generateCmd.Flags().BoolVarP(&workload, "workload", "", false, "generate workloads config")
	//
	configApplyCmd.Flags().BoolVarP(&infra, "infra", "", false, "generate infra config")
	configApplyCmd.Flags().BoolVarP(&workload, "workload", "", false, "generate workloads config")
}
