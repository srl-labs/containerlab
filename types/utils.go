package types

import (
	"fmt"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/google/uuid"
	"github.com/vishvananda/netlink"
)

// toNS assigns a veth endpoint to a given netns and renames its random name to a desired name.
func toNS(nlLink netlink.Link, nsPath string, nsLinkName string) error {
	var vethNS ns.NetNS
	var err error
	if vethNS, err = ns.GetNS(nsPath); err != nil {
		return err
	}
	// move veth endpoint to namespace
	if err = netlink.LinkSetNsFd(nlLink, int(vethNS.Fd())); err != nil {
		return err
	}
	err = vethNS.Do(func(_ ns.NetNS) error {
		if err = netlink.LinkSetName(nlLink, nsLinkName); err != nil {
			return fmt.Errorf(
				"failed to rename link: %v", err)
		}

		if err = netlink.LinkSetUp(nlLink); err != nil {
			return fmt.Errorf("failed to set %q up: %v",
				nsLinkName, err)
		}
		return nil
	})
	return err
}

func genRandomIfName() string {
	s, _ := uuid.New().MarshalText() // .MarshalText() always return a nil error
	return "clab-" + string(s[:8])
}

func extractHostNodeInterfaceData(lc LinkConfig, specialEPIndex int) (host string, hostIf string, node string, nodeIf string) {
	// the index of the node is the specialEndpointIndex +1  modulo 2
	nodeindex := (specialEPIndex + 1) % 2

	hostData := strings.SplitN(lc.Endpoints[specialEPIndex], ":", 2)
	nodeData := strings.SplitN(lc.Endpoints[nodeindex], ":", 2)

	host = hostData[0]
	hostIf = hostData[1]
	node = nodeData[0]
	nodeIf = nodeData[1]

	return host, hostIf, node, nodeIf
}
