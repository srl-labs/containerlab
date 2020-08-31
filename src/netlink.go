package main

import (
	"os/exec"
	"strconv"

	log "github.com/sirupsen/logrus"
)

func (c *cLab) createVirtualWiring(id int, link *Link) (err error) {

	nodeNameA := "lab" + "-" + c.Conf.Prefix + "-" + link.a.Node.Name
	nodeNameB := "lab" + "-" + c.Conf.Prefix + "-" + link.b.Node.Name
	log.Debug("creating veth pair: ", nodeNameA, nodeNameB, link.a.EndpointName, link.b.EndpointName)

	createDirectory("/run/netns/", 0755)

	var src, dst string

	// var ns netns.NsHandle
	// ns, err = netns.NewNamed(nodeNameA)
	// if err != nil {
	// 	log.Error(err)
	// }
	var b []byte

	log.Debug("Create link to /run/netns/ ", nodeNameA)
	src = "/proc/" + strconv.Itoa(link.a.Node.Pid) + "/ns/net"
	dst = "/run/netns/" + nodeNameA
	//err = linkFile(src, dst)
	b, err = exec.Command("sudo", "ln", "-s", src, dst).CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))

	log.Debug("Create link to /run/netns/ ", nodeNameB)
	src = "/proc/" + strconv.Itoa(link.b.Node.Pid) + "/ns/net"
	dst = "/run/netns/" + nodeNameB
	//err = linkFile(src, dst)

	b, err = exec.Command("sudo", "ln", "-s", src, dst).CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))

	log.Debug("create dummy veth pair")
	b, err = exec.Command("sudo", "ip", "link", "add", "dummyA", "type", "veth", "peer", "name", "dummyB").CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))

	log.Debug("map dummy interface on container A to NS")
	b, err = exec.Command("sudo", "ip", "link", "set", "dummyA", "netns", nodeNameA).CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))
	log.Debug("map dummy interface on container B to NS")
	b, err = exec.Command("sudo", "ip", "link", "set", "dummyB", "netns", nodeNameB).CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))
	log.Debug("rename interface container NS A")
	b, err = exec.Command("sudo", "ip", "netns", "exec", nodeNameA, "ip", "link", "set", "dummyA", "name", link.a.EndpointName).CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))
	log.Debug("rename interface container NS B")
	b, err = exec.Command("sudo", "ip", "netns", "exec", nodeNameB, "ip", "link", "set", "dummyB", "name", link.b.EndpointName).CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))
	log.Debug("set interface up in container NS A")
	b, err = exec.Command("sudo", "ip", "netns", "exec", nodeNameA, "ip", "link", "set", link.a.EndpointName, "up").CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))
	log.Debug("set interface up in container NS B")
	b, err = exec.Command("sudo", "ip", "netns", "exec", nodeNameB, "ip", "link", "set", link.b.EndpointName, "up").CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))
	log.Debug("set RX, TX offload off on container A")
	b, err = exec.Command("sudo", "docker", "exec", "-ti", nodeNameA, "ethtool", "--offload", link.a.EndpointName, "rx", "off", "tx", "off").CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	log.Debug(string(b))
	log.Debug("set RX, TX offload off on container B")
	// b, err = exec.Command("sudo", "docker", "exec", "-ti", nodeNameB, "ethtool", "--offload", link.b.EndpointName, "rx", "off", "tx", "off").CombinedOutput()
	// if err != nil {
	// 	log.Fatalf("cmd.Run() failed with %s\n", err)
	// }
	// log.Debug(string(b))
	// _, err = cmd.CombinedOutput()
	// if err != nil {
	// 	log.Fatalf("cmd.Run() failed with %s\n", err)
	// }

	//ip link add tmp_a type veth peer name tmp_b
	//ip link set tmp_a netns $srl_a
	//ip link set tmp_b netns $srl_b
	//ip netns exec $srl_a ip link set tmp_a name $srl_a_int
	//ip netns exec $srl_b ip link set tmp_b name $srl_b_int
	//ip netns exec $srl_a ip link set $srl_a_int up
	//ip netns exec $srl_b ip link set $srl_b_int up

	//docker exec -ti $srl_a ethtool --offload $srl_a_int rx off tx off
	//docker exec -ti $srl_b ethtool --offload $srl_b_int rx off tx off

	return nil

}

func (c *cLab) deleteVirtualWiring(id int, link *Link) (err error) {

	nodeNameA := "lab" + "-" + c.Conf.Prefix + "-" + link.a.Node.Name
	nodeNameB := "lab" + "-" + c.Conf.Prefix + "-" + link.b.Node.Name

	var cmd *exec.Cmd

	log.Debug("Delete netns: ", nodeNameA)
	//err = linkFile(src, dst)
	cmd = exec.Command("sudo", "ip", "netns", "del", nodeNameA)
	_, err = cmd.CombinedOutput()
	//if err != nil {
	//	log.Fatalf("cmd.Run() failed with %s\n", err)
	//}

	log.Debug("Delete netns: ", nodeNameB)
	//err = linkFile(src, dst)
	cmd = exec.Command("sudo", "ip", "netns", "del", nodeNameB)
	_, err = cmd.CombinedOutput()
	//if err != nil {
	//	log.Fatalf("cmd.Run() failed with %s\n", err)
	//}
	return nil
}