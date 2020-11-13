package clab

import (
	"fmt"
	"net"
	"os"
	"os/exec"
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
}

func (c *cLab) newLinkAttributes() LinkAttributes {
	return LinkAttributes{Name: "", MTU: 1500, LinkUp: true}
}

func (c *cLab) configVeth(dummyInterface string, la LinkAttributes, ns string) error {

	log.Debugf("Disabling TX checksum offloading for the %s interface...", dummyInterface)
	err := EthtoolTXOff(dummyInterface)
	if err != nil {
		return err
	}

	netNS, err := netns.GetFromName(ns)
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
	log.Debugf("setting interface %s MTU to %d", la.Name, la.MTU)
	netlink.LinkSetMTU(link, la.MTU)
	if err != nil {
		return err
	}
	netns.Set(hostNetNs)
	return nil
}

func (c *cLab) createvethToBridge(l *Link) error {
	var cmd *exec.Cmd
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
	cmd = exec.Command("sudo", "ip", "link", "add", dummyIface, "type", "veth", "peer", "name", bridgeIfname)
	err = runCmd(cmd)
	if err != nil {
		return err
	}

	la := c.newLinkAttributes()
	la.Name = containerIfName
	err = c.configVeth(dummyIface, la, containerNS)
	if err != nil {
		return err
	}
	log.Debugf("map veth pair %s to bridge %s", bridgeIfname, bridgeName)
	cmd = exec.Command("sudo", "ip", "link", "set", bridgeIfname, "master", bridgeName)
	err = runCmd(cmd)
	if err != nil {
		return err
	}
	log.Debugf("set interface '%s' state to up", bridgeIfname)
	cmd = exec.Command("sudo", "ip", "link", "set", bridgeIfname, "up")
	err = runCmd(cmd)
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

func runCmd(cmd *exec.Cmd) error {
	b, err := cmd.CombinedOutput()
	if err != nil {
		log.Debugf("'%s' failed with: %v", cmd.String(), err)
		log.Debugf("'%s' failed output: %v", cmd.String(), string(b))
		return err
	}
	log.Debugf("'%s' output: %v", cmd.String(), string(b))
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
