package clab

import (
	"os/exec"
	"strconv"

	log "github.com/sirupsen/logrus"
)

func (c *cLab) InitVirtualWiring() {
	log.Debug("delete dummyA eth link")
	var cmd *exec.Cmd
	var err error
	cmd = exec.Command("sudo", "ip", "link", "del", "dummyA")
	err = runCmd(cmd)
	if err != nil {
		log.Debugf("%s failed with: %v", cmd.String(), err)
	}
	cmd = exec.Command("sudo", "ip", "link", "del", "dummyB")
	err = runCmd(cmd)
	if err != nil {
		log.Debugf("%s failed with: %v", cmd.String(), err)
	}
}

// CreateVirtualWiring provides the virtual topology between the containers
func (c *cLab) CreateVirtualWiring(id int, link *Link) (err error) {
	log.Infof("Create virtual wire : %s, %s, %s, %s", link.A.Node.LongName, link.B.Node.LongName, link.A.EndpointName, link.B.EndpointName)


	CreateDirectory("/run/netns/", 0755)

	var src, dst string
	var cmd *exec.Cmd

	if link.A.Node.Kind != "bridge" { // if the node is not a bridge
		log.Debug("Create link to /run/netns/ ", link.A.Node.LongName)
		src = "/proc/" + strconv.Itoa(link.A.Node.Pid) + "/ns/net"
		dst = "/run/netns/" + link.A.Node.LongName
		//err = linkFile(src, dst)
		cmd = exec.Command("sudo", "ln", "-s", src, dst)
		err = runCmd(cmd)
		if err != nil {
			log.Debugf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.B.Node.Kind != "bridge" { // if the node is not a bridge
		log.Debug("Create link to /run/netns/ ", link.B.Node.LongName)
		src = "/proc/" + strconv.Itoa(link.B.Node.Pid) + "/ns/net"
		dst = "/run/netns/" + link.B.Node.LongName
		//err = linkFile(src, dst)
		cmd = exec.Command("sudo", "ln", "-s", src, dst)
		err = runCmd(cmd)
		if err != nil {
			log.Debugf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.A.Node.Kind != "bridge" && link.B.Node.Kind != "bridge" { // none of the 2 nodes is a bridge
		log.Debug("create dummy veth pair")
		cmd = exec.Command("sudo", "ip", "link", "add", "dummyA", "type", "veth", "peer", "name", "dummyB")
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	} else if link.A.Node.Kind != "bridge" { // node of link A is a bridge
		log.Debug("create dummy veth pair")
		cmd = exec.Command("sudo", "ip", "link", "add", "dummyA", "type", "veth", "peer", "name", link.B.EndpointName)
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	} else { // node of link B is a bridge
		log.Debug("create dummy veth pair")
		cmd = exec.Command("sudo", "ip", "link", "add", "dummyB", "type", "veth", "peer", "name", link.A.EndpointName)
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.A.Node.Kind != "bridge" { // if node A of link is not a bridge
		log.Debug("map dummy interface on container A to NS")
		cmd = exec.Command("sudo", "ip", "link", "set", "dummyA", "netns", link.A.Node.LongName)
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.B.Node.Kind != "bridge" { // if node B of link is not a bridge
		log.Debug("map dummy interface on container B to NS")
		cmd = exec.Command("sudo", "ip", "link", "set", "dummyB", "netns", link.B.Node.LongName)
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.A.Node.Kind != "bridge" { // if node A of link is not a bridge
		log.Debug("rename interface container NS A")
		cmd = exec.Command("sudo", "ip", "netns", "exec", link.A.Node.LongName, "ip", "link", "set", "dummyA", "name", link.A.EndpointName)
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	} else {
		log.Debug("map veth pair to bridge")
		cmd = exec.Command("sudo", "ip", "link", "set", link.A.EndpointName, "master", link.A.Node.ShortName)
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.B.Node.Kind != "bridge" { // if node B of link is not a bridge
		log.Debug("rename interface container NS B")
		cmd = exec.Command("sudo", "ip", "netns", "exec", link.B.Node.LongName, "ip", "link", "set", "dummyB", "name", link.B.EndpointName)
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	} else {
		log.Debug("map veth pair to bridge")
		cmd = exec.Command("sudo", "ip", "link", "set", link.B.EndpointName, "master", link.B.Node.ShortName)
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.A.Node.Kind != "bridge" { // if node A of link is not a bridge
		log.Debug("set interface up in container NS A")
		cmd = exec.Command("sudo", "ip", "netns", "exec", link.A.Node.LongName, "ip", "link", "set", link.A.EndpointName, "up")
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	} else {
		log.Debug("set interface up in bridge")
		cmd = exec.Command("sudo", "ip", "link", "set", link.A.EndpointName, "up")
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.B.Node.Kind != "bridge" { // if node B of link is not a bridge
		log.Debug("set interface up in container NS B")
		cmd = exec.Command("sudo", "ip", "netns", "exec", link.B.Node.LongName, "ip", "link", "set", link.B.EndpointName, "up")
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	} else {
		log.Debug("set interface up in bridge")
		cmd = exec.Command("sudo", "ip", "link", "set", link.B.EndpointName, "up")
		err = runCmd(cmd)
		if err != nil {
			log.Fatalf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.A.Node.Kind != "bridge" { // if node A of link is not a bridge
		log.Debug("set RX, TX offload off on container A")
		cmd = exec.Command("docker", "exec", link.A.Node.LongName, "ethtool", "--offload", link.A.EndpointName, "rx", "off", "tx", "off")
		err = runCmd(cmd)
		if err != nil {
			log.Debugf("%s failed with: %v", cmd.String(), err)
		}
	} else {
		log.Debug("set RX, TX offload off on veth of the bridge interface")
		cmd = exec.Command("sudo", "ethtool", "--offload", link.A.EndpointName, "rx", "off", "tx", "off")
		err = runCmd(cmd)
		if err != nil {
			log.Debugf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.B.Node.Kind != "bridge" { // if node B of link is not a bridge
		log.Debug("set RX, TX offload off on container B")
		cmd = exec.Command("docker", "exec", link.B.Node.LongName, "ethtool", "--offload", link.B.EndpointName, "rx", "off", "tx", "off")
		err = runCmd(cmd)
		if err != nil {
			log.Debugf("%s failed with: %v", cmd.String(), err)
		}
	} else {
		log.Debug("set RX, TX offload off on veth of the bridge interface")
		cmd = exec.Command("sudo", "ethtool", "--offload", link.B.EndpointName, "rx", "off", "tx", "off")
		err = runCmd(cmd)
		if err != nil {
			log.Debugf("%s failed with: %v", cmd.String(), err)
		}
	}

	//sudo ip link add tmp_a type veth peer name tmp_b
	//sudo ip link set tmp_a netns $srl_a
	//ip link set tmp_b netns $srl_b
	//ip netns exec $srl_a ip link set tmp_a name $srl_a_int
	//ip netns exec $srl_b ip link set tmp_b name $srl_b_int
	//ip netns exec $srl_a ip link set $srl_a_int up
	//ip netns exec $srl_b ip link set $srl_b_int up
	//docker exec -ti $srl_a ethtool --offload $srl_a_int rx off tx off
	//docker exec -ti $srl_b ethtool --offload $srl_b_int rx off tx off

	//sudo ip link add <bridge-name> type bridge
	//sudo ip link set <bridge-name> up

	//sudo ip link add tmp_a type veth peer name <vethint>
	//sudo ip link set tmp_a netns <container>
	//sudo ip netns exec <container> ip link set tmp_a name e1-10
	//sudo ip netns exec <container> ip link set e1-10 up
	//sudo ip link set <vethint> master <bridge-name>
	//sudo ip link set <vethint> up
	//docker exec -ti <container> --offload $srl_a_int rx off tx off
	//sudo ethtool --offload <vethint> rx off tx off

	return nil

}

// DeleteVirtualWiring deletes the virtual wiring
func (c *cLab) DeleteVirtualWiring(id int, link *Link) (err error) {
	log.Info("Delete virtual wire :", link.A.Node.ShortName, link.B.Node.ShortName, link.A.EndpointName, link.B.EndpointName)

	var cmd *exec.Cmd

	if link.A.Node.Kind != "bridge" {
		log.Debug("Delete netns: ", link.A.Node.LongName)
		cmd = exec.Command("sudo", "ip", "netns", "del", link.A.Node.LongName)
		err = runCmd(cmd)
		if err != nil {
			log.Debugf("%s failed with: %v", cmd.String(), err)
		}
	}

	if link.B.Node.Kind != "bridge" {
		log.Debug("Delete netns: ", link.B.Node.LongName)
		cmd = exec.Command("sudo", "ip", "netns", "del", link.B.Node.LongName)
		err = runCmd(cmd)
		if err != nil {
			log.Debugf("%s failed with: %v", cmd.String(), err)
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
