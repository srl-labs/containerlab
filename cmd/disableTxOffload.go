package cmd

import (
	"context"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/utils"
)

var cntName string

// upgradeCmd represents the version command
var disableTxOffloadCmd = &cobra.Command{
	Use:   "disable-tx-offload",
	Short: "disables tx checksum offload on eth0 interface of a container",

	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithRuntime(rt, debug, timeout, graceful),
		}
		c := clab.NewContainerLab(opts...)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		log.Infof("getting container '%s' information", cntName)

		NSPath, err := c.Runtime.GetNSPath(ctx, cntName)
		if err != nil {
			return err
		}
		nodeNS, err := ns.GetNS(NSPath)
		if err != nil {
			return err
		}
		err = nodeNS.Do(func(_ ns.NetNS) error {
			// disabling offload on lo0 interface
			err = utils.EthtoolTXOff("eth0")
			if err != nil {
				log.Infof("Failed to disable TX checksum offload for 'eth0' interface for '%s' container", cntName)
			}
			return nil
		})
		if err != nil {
			return err
		}
		log.Infof("Tx checksum offload disabled for eth0 interface of %s container", cntName)
		return nil
	},
}

func init() {
	toolsCmd.AddCommand(disableTxOffloadCmd)
	disableTxOffloadCmd.Flags().StringVarP(&cntName, "container", "c", "", "container name to disable offload in")
	_ = disableTxOffloadCmd.MarkFlagRequired("container")
}
