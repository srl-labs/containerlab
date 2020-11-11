package cmd

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	rootCaCsrTemplate = "/etc/containerlab/templates/ca/csr-root-ca.json"
	certCsrTemplate   = "/etc/containerlab/templates/ca/csr.json"
)

var debug bool
var timeout time.Duration

// path to the topology file
var topo string
var graph bool

// lab prefix
var prefix string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "containerlab",
	Short: "deploy container based lab environments with a user-defined interconnections",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if debug {
			log.SetLevel(log.DebugLevel)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	id := os.Geteuid()
	if id != 0 {
		fmt.Println("containerlab requires sudo privileges to run!")
		os.Exit(1)
	}

	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
	rootCmd.PersistentFlags().StringVarP(&topo, "topo", "t", "", "path to the file with topology information")
	rootCmd.PersistentFlags().StringVarP(&prefix, "prefix", "p", "", "lab name prefix")
	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "", 30*time.Second, "timeout for docker requests, e.g: 30s, 1m, 2m30s")

}
