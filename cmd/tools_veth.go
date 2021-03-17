package cmd

import (
	"context"
	"errors"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

var AEnd = ""
var BEnd = ""
var Mtu = 1500

func init() {
	toolsCmd.AddCommand(vethCmd)
	vethCmd.AddCommand(vethCreateCmd)
	vethCreateCmd.Flags().StringVarP(&AEnd, "aend", "a", "", "<name of container1>:<interface name>")
	vethCreateCmd.Flags().StringVarP(&BEnd, "bend", "b", "", "<name of container2>:<interface name>")
	vethCreateCmd.Flags().IntVarP(&Mtu, "mtu", "m", Mtu, "MTU of the link")
}

var vethCmd = &cobra.Command{
	Use:   "veth",
	Short: "veth operations",
}

var vethCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "On-the-fly create a veth pair and attach it to the specified namespaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		opts := []clab.ClabOption{
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithEnvDockerClient(),
		}
		c := clab.NewContainerLab(opts...)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var aEndCif containerInterface
		var bEndCif containerInterface

		if aEndCif, err = checkContainerInterfaceRef(AEnd); err != nil {
			return err
		}
		if bEndCif, err = checkContainerInterfaceRef(BEnd); err != nil {
			return err
		}

		aEndNode := new(clab.Node)
		aEndNode.LongName = aEndCif.container
		aEndNode.ShortName = aEndCif.container

		bEndNode := new(clab.Node)
		bEndNode.LongName = bEndCif.container
		bEndNode.ShortName = bEndCif.container

		nsPathA, err := c.GetNSPath(ctx, aEndNode.LongName)
		if err != nil {
			return err
		}
		nsPathB, err := c.GetNSPath(ctx, bEndNode.LongName)
		if err != nil {
			return err
		}

		aEndNode.NSPath = nsPathA
		bEndNode.NSPath = nsPathB

		endpointA := clab.Endpoint{
			Node:         aEndNode,
			EndpointName: aEndCif.iface,
		}
		endpointB := clab.Endpoint{
			Node:         bEndNode,
			EndpointName: bEndCif.iface,
		}

		link := new(clab.Link)
		link.A = &endpointA
		link.B = &endpointB
		link.MTU = Mtu

		if err := c.CreateVirtualWiring(link); err != nil {
			return err
		}
		log.Info("veth pair successfully created!")
		return nil
	},
}

func checkContainerInterfaceRef(s string) (containerInterface, error) {
	cif := *new(containerInterface)
	arr := strings.Split(s, ":")
	if len(arr) != 2 {
		return cif, errors.New("malformed container interface reference")
	}
	cif.container = arr[0]
	cif.iface = arr[1]

	return cif, nil
}

type containerInterface struct {
	container string
	iface     string
}
