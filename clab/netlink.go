package clab

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func (c *cLab) InitVirtualWiring() {
	// list interfaces
	log.Debug("listing system interfaces...")
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Warnf("failed to get system interfaces:%v", err)
		return
	}
	log.Debugf("found %d interfaces", len(interfaces))
	for i := range interfaces {
		if strings.HasPrefix(interfaces[i].Name, "clab-") {
			log.Debugf("deleting interface %s", interfaces[i].Name)
			l, err := netlink.LinkByName(interfaces[i].Name)
			if err != nil {
				log.Debugf("failed to find interface for deletion by name: %v", interfaces[i].Name)
				continue
			}
			err = netlink.LinkDel(l)
			if err != nil {
				log.Debugf("failed to delete interface %s: %v", interfaces[i].Name, err)
			}
		}
	}
}

// CreateVirtualWiring provides the virtual topology between the containers
func (c *cLab) CreateVirtualWiring(link *Link) (err error) {
	log.Infof("Create virtual wire : %s:%s <--> %s:%s", link.A.Node.LongName, link.A.EndpointName, link.B.Node.LongName, link.B.EndpointName)
	if link.A.Node.Kind != "bridge" && link.B.Node.Kind != "bridge" {
		return c.createAToBveth(link)
	}
	return c.createvethToBridge(link)
}

func (c *cLab) createAToBveth(l *Link) error {
	interfaceA := fmt.Sprintf("clab-%s", genIfName())
	interfaceB := fmt.Sprintf("clab-%s", genIfName())

	nllA := &netlink.Veth{PeerName: interfaceB, LinkAttrs: netlink.LinkAttrs{Name: interfaceA}}

	err := netlink.LinkAdd(nllA)
	if err != nil {
		return err
	}

	la := c.newLinkAttributes()
	la.Name = l.A.EndpointName
	err = c.configVeth(interfaceA, la, l.A.Node.LongName)
	if err != nil {
		log.Fatalf("failed to config interface '%s' in container %s: %v", l.A.EndpointName, l.A.Node.LongName, err)
	}

	la = c.newLinkAttributes()
	la.Name = l.B.EndpointName
	err = c.configVeth(interfaceB, la, l.B.Node.LongName)
	if err != nil {
		log.Fatalf("failed to config interface '%s' in container %s: %v", l.B.EndpointName, l.B.Node.LongName, err)
	}

	return nil
}

// Type for the adjustment of the Link Attributes
type LinkAttributes struct {
	Name   string
	MTU    int
	LinkUp bool
	// Master is just used for interfaces meant to be attached to bridges
	Master string
}

// Initialize and instantiate a new LinkAttributes with default values
func (c *cLab) newLinkAttributes() LinkAttributes {
	return LinkAttributes{Name: "", MTU: 1500, LinkUp: true, Master: ""}
}

func (c *cLab) configVeth(dummyInterface string, la LinkAttributes, ns string) error {

	netNS, err := netns.GetFromName(ns)
	if err != nil {
		return err
	}

	log.Debugf("Disabling TX checksum offloading for the %s interface...", dummyInterface)
	err = EthtoolTXOff(dummyInterface)
	if err != nil {
		return err
	}

	netLink, err := netlink.LinkByName(dummyInterface)
	if err != nil {
		return err
	}

	log.Debugf("map dummy interface '%s' to container %s", dummyInterface, ns)
	err = netlink.LinkSetNsFd(netLink, int(netNS))
	if err != nil {
		return err
	}

	err = c.setLinkAttributes(ns, netNS, dummyInterface, la)
	if err != nil {
		return err
	}
	return nil

}

func (c *cLab) setLinkAttributes(namespaceName string, cnamespace netns.NsHandle, oldLinkName string, la LinkAttributes) error {
	hostNetNs, _ := netns.Get()
	netns.Set(cnamespace)
	runtime.LockOSThread()

	link, err := netlink.LinkByName(oldLinkName)
	if err != nil {
		return err
	}
	if la.Name != "" {
		log.Debugf("rename interface %s to %s", oldLinkName, la.Name)
		err = netlink.LinkSetName(link, la.Name)
		if err != nil {
			return err
		}
	}
	if la.LinkUp {
		log.Debugf("set interface %s state to up in NS %s", la.Name, namespaceName)
		err = netlink.LinkSetUp(link)
		if err != nil {
			return err
		}
	}
	if la.Master != "" {
		// if interface should be attached to a bridge, the Master is set to bridge name
		log.Debugf("attache interface %s to bridge %s", la.Name, la.Master)

		// get handleto master link
		master, err := netlink.LinkByName(la.Master)
		if err != nil {
			return err
		}
		// assigne master for link
		err = netlink.LinkSetMaster(link, master)
		if err != nil {
			return err
		}
	}
	log.Debugf("setting interface %s MTU to %d", la.Name, la.MTU)
	netlink.LinkSetMTU(link, la.MTU)
	if err != nil {
		return err
	}
	netns.Set(hostNetNs)
	return nil
}

func (c *cLab) createvethToBridge(l *Link) error {
	var err error
	log.Debugf("Create veth to bridge wire: %s <--> %s", l.A.EndpointName, l.B.EndpointName)
	dummyIface := fmt.Sprintf("clab-%s", genIfName())
	// assume A is a bridge
	bridgeName := l.A.Node.ShortName
	bridgeIfname := l.A.EndpointName

	containerIfName := l.B.EndpointName
	containerNS := l.B.Node.LongName

	if l.A.Node.Kind != "bridge" { // change var values if A is not a bridge
		bridgeName = l.B.Node.ShortName
		bridgeIfname = l.B.EndpointName

		containerIfName = l.A.EndpointName
		containerNS = l.A.Node.LongName
	}

	log.Debugf("create dummy veth pair '%s'<-->'%s'", dummyIface, bridgeIfname)
	nllA := &netlink.Veth{PeerName: bridgeIfname, LinkAttrs: netlink.LinkAttrs{Name: dummyIface}}

	err = netlink.LinkAdd(nllA)
	if err != nil {
		return err
	}

	la := c.newLinkAttributes()
	la.Name = containerIfName
	la.Master = bridgeName
	err = c.configVeth(dummyIface, la, containerNS)
	if err != nil {
		return err
	}

	log.Debugf("Disabling TX checksum offloading for the %s interface...", bridgeIfname)
	err = EthtoolTXOff(bridgeIfname)
	if err != nil {
		return err
	}
	return nil
}

// DeleteNetnsSymlinks deletes the symlink file created for each container netns
func (c *cLab) DeleteNetnsSymlinks() (err error) {
	for _, node := range c.Nodes {
		if node.Kind != "bridge" {
			log.Infof("Deleting %s network namespace", node.LongName)
			if err := deleteNetnsSymlink(node.LongName); err != nil {
				return err
			}
		}

	}

	return nil
}

func genIfName() string {
	s, _ := uuid.New().MarshalText() // .MarshalText() always return a nil error
	return string(s[:8])
}

// deleteNetnsSymlink deletes a network namespace and removes the symlink created by linkContainerNS func
func deleteNetnsSymlink(n string) error {
	log.Debug("Deleting netns symlink: ", n)
	sl := fmt.Sprintf("/run/netns/%s", n)
	err := os.Remove(sl)
	if err != nil {
		log.Debug("Failed to delete netns symlink by path:", sl)
	}
	return nil
}
