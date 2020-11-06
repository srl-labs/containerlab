package clab

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
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

	cmd := exec.Command("sudo", "ip", "link", "add", interfaceA, "type", "veth", "peer", "name", interfaceB)
	err := runCmd(cmd)
	if err != nil {
		return err
	}
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := c.configVeth(interfaceA, l.A.EndpointName, l.A.Node.LongName)
		if err != nil {
			log.Fatalf("failed to config interface '%s' in container %s: %v", l.A.EndpointName, l.A.Node.LongName, err)
		}
	}()
	go func() {
		defer wg.Done()
		err = c.configVeth(interfaceB, l.B.EndpointName, l.B.Node.LongName)
		if err != nil {
			log.Fatalf("failed to config interface '%s' in container %s: %v", l.B.EndpointName, l.B.Node.LongName, err)
		}
	}()
	wg.Wait()
	return nil
}

func (c *cLab) configVeth(dummyInterface, endpointName, ns string) error {
	var cmd *exec.Cmd
	log.Debugf("Disabling TX checksum offloading for the %s interface...", dummyInterface)
	err := EthtoolTXOff(dummyInterface)
	if err != nil {
		return err
	}
	log.Debugf("map dummy interface '%s' to container %s", dummyInterface, ns)
	cmd = exec.Command("sudo", "ip", "link", "set", dummyInterface, "netns", ns)
	err = runCmd(cmd)
	if err != nil {
		return err
	}
	log.Debugf("rename interface %s to %s", dummyInterface, endpointName)
	cmd = exec.Command("sudo", "ip", "netns", "exec", ns, "ip", "link", "set", dummyInterface, "name", endpointName)
	err = runCmd(cmd)
	if err != nil {
		return err
	}
	log.Debugf("set interface %s state to up in NS %s", endpointName, ns)
	cmd = exec.Command("sudo", "ip", "netns", "exec", ns, "ip", "link", "set", endpointName, "up")
	err = runCmd(cmd)
	if err != nil {
		return err
	}
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
	err = c.configVeth(dummyIface, containerIfName, containerNS)
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
