package config

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab"
	"inet.af/netaddr"
)

const (
	systemIP = "systemip"
)

func linkIPfromSystemIP(link *clab.Link) (netaddr.IPPrefix, netaddr.IPPrefix, error) {
	var ipA netaddr.IPPrefix
	var err error
	if linkIp, ok := link.Labels["ip"]; ok {
		// calc far end IP
		ipA, err = netaddr.ParseIPPrefix(linkIp)
		if err != nil {
			return ipA, ipA, fmt.Errorf("invalid ip %s", link.A.EndpointName)
		}
	} else {
		// caluculate link IP from the system IPs - tbd
		//var sysA, sysB netaddr.IPPrefix

		sysA, err := netaddr.ParseIPPrefix(link.A.Node.Labels[systemIP])
		if err != nil {
			return ipA, ipA, fmt.Errorf("no 'ip' on link & the '%s' of %s: %s", systemIP, link.A.Node.ShortName, err)
		}
		sysB, err := netaddr.ParseIPPrefix(link.B.Node.Labels[systemIP])
		if err != nil {
			return ipA, ipA, fmt.Errorf("no 'ip' on link & the '%s' of %s: %s", systemIP, link.B.Node.ShortName, err)
		}
		o2, o3, o4 := ipLastOctet(sysA.IP), ipLastOctet(sysB.IP), 0
		if o3 < o2 {
			o2, o3, o4 = o3, o2, o4+1
		}
		ipA, err = netaddr.ParseIPPrefix(fmt.Sprintf("1.%d.%d.%d/31", o2, o3, o4))
		if err != nil {
			log.Errorf("could not create link IP from system-ip: %s", err)
		}
	}
	return ipA, ipFarEnd(ipA), nil
}

func ipLastOctet(in netaddr.IP) int {
	s := in.String()
	i := strings.LastIndexAny(s, ".")
	if i < 0 {
		i = strings.LastIndexAny(s, ":")
	}
	res, err := strconv.Atoi(s[i+1:])
	if err != nil {
		log.Errorf("last octect %s from IP %s not a string", s[i+1:], s)
	}
	return res
}

func ipFarEnd(in netaddr.IPPrefix) netaddr.IPPrefix {
	if in.IP.Is4() && in.Bits == 32 {
		return netaddr.IPPrefix{}
	}

	n := in.IP.Next()

	if in.IP.Is4() && in.Bits <= 30 {
		if !in.Contains(n) || !in.Contains(in.IP.Prior()) {
			return netaddr.IPPrefix{}
		}
		if !in.Contains(n.Next()) {
			n = in.IP.Prior()
		}
	}
	if !in.Contains(n) {
		n = in.IP.Prior()
	}
	if !in.Contains(n) {
		return netaddr.IPPrefix{}
	}
	return netaddr.IPPrefix{
		IP:   n,
		Bits: in.Bits,
	}
}
