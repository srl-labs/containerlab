package config

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"inet.af/netaddr"
)

const (
	systemIP = "systemip"
)

type Dict map[string]interface{}

func PrepareVars(nodes map[string]*types.Node, links map[int]*types.Link) map[string]Dict {

	res := make(map[string]Dict)

	// preparing all nodes vars
	for _, node := range nodes {
		name := node.ShortName
		// Init array for this node
		res[name] = make(map[string]interface{})
		nc := GetNodeConfigFromLabels(node.Labels)
		for key := range nc.Vars {
			res[name][key] = nc.Vars[key]
		}
		// Create link array
		res[name]["links"] = []interface{}{}
		// Ensure role or Kind
		if _, ok := res[name]["role"]; !ok {
			res[name]["role"] = node.Kind
		}
	}

	// prepare all links
	for lIdx, link := range links {
		varsA := make(map[string]interface{})
		varsB := make(map[string]interface{})
		err := prepareLinkVars(lIdx, link, varsA, varsB)
		if err != nil {
			log.Errorf("cannot prepare link vars for %d. %s: %s", lIdx, link.String(), err)
		}
		res[link.A.Node.ShortName]["links"] = append(res[link.A.Node.ShortName]["links"].([]interface{}), varsA)
		res[link.B.Node.ShortName]["links"] = append(res[link.B.Node.ShortName]["links"].([]interface{}), varsB)
	}
	return res
}

func prepareLinkVars(lIdx int, link *types.Link, varsA, varsB map[string]interface{}) error {
	ncA := GetNodeConfigFromLabels(link.A.Node.Labels)
	ncB := GetNodeConfigFromLabels(link.B.Node.Labels)
	linkVars := link.Labels

	addV := func(key string, v1 interface{}, v2 ...interface{}) {
		varsA[key] = v1
		if len(v2) == 0 {
			varsB[key] = v1
		} else {
			varsA[key+"_far"] = v2[0]
			varsB[key] = v2[0]
			varsB[key+"_far"] = v1
		}
	}

	// Link IPs
	ipA, ipB, err := linkIPfromSystemIP(link)
	if err != nil {
		return fmt.Errorf("%s: %s", link, err)
	}
	addV("ip", ipA.String(), ipB.String())
	addV(systemIP, ncA.Vars[systemIP], ncB.Vars[systemIP])

	// Split all fields with a comma...
	for k, v := range linkVars {
		r := SplitTrim(v)
		switch len(r) {
		case 1:
			addV(k, r[0])
		case 2:
			addV(k, r[0], r[1])
		default:
			log.Warnf("%s: %s contains %d elements, should be 1 or 2: %s", link.String(), k, len(r), v)
		}
	}

	if _, ok := varsA["name"]; !ok {
		var linkNr string
		if v, ok := varsA["linkNr"]; ok {
			linkNr = fmt.Sprintf("_%v", v)
		}
		addV("name", fmt.Sprintf("to_%s%s", link.B.Node.ShortName, linkNr),
			fmt.Sprintf("to_%s%s", link.A.Node.ShortName, linkNr))
	}

	return nil
}

func linkIPfromSystemIP(link *types.Link) (netaddr.IPPrefix, netaddr.IPPrefix, error) {
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
		o2, o3, o4 := ipLastOctet(sysA.IP()), ipLastOctet(sysB.IP()), 0
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
	if in.IP().Is4() && in.Bits() == 32 {
		return netaddr.IPPrefix{}
	}

	n := in.IP().Next()

	if in.IP().Is4() && in.Bits() <= 30 {
		if !in.Contains(n) || !in.Contains(in.IP().Prior()) {
			return netaddr.IPPrefix{}
		}
		if !in.Contains(n.Next()) {
			n = in.IP().Prior()
		}
	}
	if !in.Contains(n) {
		n = in.IP().Prior()
	}
	if !in.Contains(n) {
		return netaddr.IPPrefix{}
	}
	return netaddr.IPPrefixFrom(n, in.Bits())
}

// DictString
