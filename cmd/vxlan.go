package cmd

import (
	"net"

	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

var vxlanRemote string
var cntLink string
var parentDev string
var vxlanMTU int
var vxlanID int

func init() {
	toolsCmd.AddCommand(vxlanCmd)
	vxlanCmd.AddCommand(vxlanCreateCmd)
	vxlanCreateCmd.Flags().IntVarP(&vxlanID, "id", "i", 10, "VxLAN ID (VNI)")
	vxlanCreateCmd.Flags().StringVarP(&vxlanRemote, "remote", "r", "", "address of the remote VTEP")
	vxlanCreateCmd.Flags().StringVarP(&parentDev, "dev", "", "eth0", "parent (source) interface name for VxLAN")
	vxlanCreateCmd.Flags().StringVarP(&cntLink, "link", "l", "", "link to which 'attach' vxlan tunnel with tc redirect")
	vxlanCreateCmd.Flags().IntVarP(&vxlanMTU, "mtu", "m", 1554, "VxLAN MTU")

	vxlanCreateCmd.MarkFlagRequired("dev")
	vxlanCreateCmd.MarkFlagRequired("remote")
	vxlanCreateCmd.MarkFlagRequired("id")
	vxlanCreateCmd.MarkFlagRequired("link")
}

// vxlanCmd represents the vxlan command container
var vxlanCmd = &cobra.Command{
	Use:   "vxlan",
	Short: "VxLAN interface commands",
}

var vxlanCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create vxlan interface",
	RunE: func(cmd *cobra.Command, args []string) error {
		vxlanCfg := clab.VxLAN{
			Name:     "vx-" + cntLink,
			ID:       vxlanID,
			ParentIf: parentDev,
			Remote:   net.ParseIP(vxlanRemote),
			MTU:      vxlanMTU,
		}

		if err := clab.AddVxLanInterface(vxlanCfg); err != nil {
			return err
		}

		if err := clab.BindIfacesWithTC(vxlanCfg.Name, cntLink); err != nil {
			return err
		}

		return nil
	},
}
