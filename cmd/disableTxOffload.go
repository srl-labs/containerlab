package cmd

import (
	"context"
	"strconv"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
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
			clab.WithEnvDockerClient(),
		}
		c := clab.NewContainerLab(opts...)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		cnt, err := c.DockerClient.ContainerInspect(ctx, cntName)
		if err != nil {
			return err
		}

		log.Infof("getting container '%s' information", cntName)
		NSPath := "/proc/" + strconv.Itoa(cnt.State.Pid) + "/ns/net"
		nodeNS, err := ns.GetNS(NSPath)
		if err != nil {
			return err
		}
		err = nodeNS.Do(func(_ ns.NetNS) error {
			// disabling offload on lo0 interface
			err = clab.EthtoolTXOff("eth0")
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
	disableTxOffloadCmd.MarkFlagRequired("container")
}
