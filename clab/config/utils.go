package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"inet.af/netaddr"
)

const (
	vkNodeName = "clab_node"  // reserved, used for the node's ShortName
	vkNodes    = "clab_nodes" // reserved, used for all nodes
	vkLinks    = "clab_links" // reserved, used for all link in a node
	vkFarEnd   = "clab_far"   // reserved, used for far-end link & node info
	vkRole     = "clab_role"  // optional, will default to the node's Kind. Used to select the template

	vkSystemIP = "clab_system_ip" // optional, system IP if present could be used to calc link IPs
	vkLinkIP   = "clab_link_ip"   // optional, link IP
	vkLinkName = "clab_link_name" // optional, from ShortNames
	vkLinkNum  = "clab_link_num"  // optional, link number in case you have multiple, used to calculate the name
)

type Dict map[string]interface{}

// Prepare variables for all nodes. This will also prepare all variables for the links
func PrepareVars(nodes map[string]nodes.Node, links map[int]*types.Link) map[string]Dict {

	res := make(map[string]Dict)

	// preparing all nodes vars
	for _, node := range nodes {
		nodeCfg := node.Config()
		name := nodeCfg.ShortName
		vars := make(Dict)
		vars[vkNodeName] = name

		// Init array for this node
		for key, val := range nodeCfg.Config.Vars {
			if key == vkNodes || key == vkNodeName {
				log.Warningf("the variable %s on %s will be ignored, it hides other nodes", vkNodes, name)
				continue
			}
			vars[key] = val
		}

		// Create link array
		vars[vkLinks] = []interface{}{}

		// Ensure role or Kind
		if _, ok := vars[vkRole]; !ok {
			vars[vkRole] = nodeCfg.Kind
		}

		res[name] = vars
	}

	// prepare all links
	for lIdx, link := range links {
		varsA := make(Dict)
		varsB := make(Dict)
		err := prepareLinkVars(lIdx, link, varsA, varsB)
		if err != nil {
			log.Errorf("cannot prepare link vars for %d. %s: %s", lIdx, link.String(), err)
		}
		res[link.A.Node.ShortName]["links"] = append(res[link.A.Node.ShortName]["links"].([]interface{}), varsA)
		res[link.B.Node.ShortName]["links"] = append(res[link.B.Node.ShortName]["links"].([]interface{}), varsB)
	}

	// Prepare top-level map of nodes
	// copy 1-level deep
	all_nodes := make(Dict)
	for name, vars := range res {
		n := make(Dict)
		all_nodes[name] = n
		for k, v := range vars {
			n[k] = v
		}
		vars[vkNodes] = all_nodes
	}
	return res
}

// Prepare variables for a specific link
func prepareLinkVars(lIdx int, link *types.Link, varsA, varsB Dict) error {

	// Add a Dict for the far-end link vars and the far-end node name
	varsA[vkFarEnd] = Dict{vkNodeName: link.B.Node.ShortName}
	varsB[vkFarEnd] = Dict{vkNodeName: link.A.Node.ShortName}

	// Add a key/value(s) pairs to the links vars (varsA & varsB)
	// If multiple vars are specified, each links also gets the far end value
	addValues := func(key string, v1 interface{}, v2 interface{}) {
		varsA[key] = v1
		(varsA[vkFarEnd]).(Dict)[key] = v2
		varsB[key] = v2
		(varsB[vkFarEnd]).(Dict)[key] = v1
	}

	// Ensure values are added to both ends of the link
	for k, v := range link.Vars {
		if k == vkFarEnd || k == vkNodeName {
			return fmt.Errorf("%s: reserved variable name '%s' found", link.String(), k)
		}

		vv := reflect.ValueOf(v)
		if vv.Kind() == reflect.Slice || vv.Kind() == reflect.Array {
			// Array/slice should contain 2 values, one for each end of the link
			if vv.Len() != 2 {
				return fmt.Errorf("%s: variable %s should contain 2 elements, found %d: %v", link.String(), k, vv.Len(), v)
			}
			addValues(k, vv.Index(0).Interface(), vv.Index(1).Interface())
			continue
		}

		if k == vkLinkIP {
			// Calculate the remote IP
			vs := fmt.Sprintf("%v", v)
			ipF, err := ipFarEndS(vs)
			if err != nil {
				return fmt.Errorf("%s: %s", link.String(), err)
			}
			addValues(k, vs, ipF)
			continue
		}

		addValues(k, v, v)
	}

	// Add additional values if they are not present
	add := map[string]func(link *types.Link) (string, string, error){
		vkLinkIP:   linkIP,
		vkLinkName: linkName,
	}

	for k, f := range add {
		if _, ok := varsA[k]; ok {
			continue
		}
		a, b, err := f(link)
		if err != nil {
			return fmt.Errorf("%s: %s", link, err)
		}
		if a != "" {
			addValues(k, a, b)
		}
	}

	return nil
}

// Create a link name using the node names and optional link_num
func linkName(link *types.Link) (string, string, error) {
	var linkNo string
	if v, ok := link.Vars[vkLinkNum]; ok {
		linkNo = fmt.Sprintf("_%v", v)
	}
	return fmt.Sprintf("to_%s%s", link.B.Node.ShortName, linkNo), fmt.Sprintf("to_%s%s", link.A.Node.ShortName, linkNo), nil
}

// Calculate link IP from the system IPs at both ends
func linkIP(link *types.Link) (string, string, error) {
	var ipA netaddr.IPPrefix
	var err error
	//
	_, okA := link.A.Node.Config.Vars[vkSystemIP]
	_, okB := link.B.Node.Config.Vars[vkSystemIP]
	if okA != okB {
		log.Warnf("to auto-generate link IPs, a %s variable is required on all nodes", vkSystemIP)
	}
	if !okA || !okB {
		return "", "", nil
	}
	sysA, err := netaddr.ParseIPPrefix(fmt.Sprintf("%v", link.A.Node.Config.Vars[vkSystemIP]))
	if err != nil {
		return "", "", fmt.Errorf("no 'ip' on link & the '%s' of %s: %s", vkSystemIP, link.A.Node.ShortName, err)
	}
	sysB, err := netaddr.ParseIPPrefix(fmt.Sprintf("%v", link.B.Node.Config.Vars[vkSystemIP]))
	if err != nil {
		return "", "", fmt.Errorf("no 'ip' on link & the '%s' of %s: %s", vkSystemIP, link.B.Node.ShortName, err)
	}

	o4 := 0
	if v, ok := link.Vars[vkLinkNum]; ok {
		o4, err = strconv.Atoi(fmt.Sprintf("%v", v))
		if err != nil {
			log.Warnf("%s is expected to contain a number, got %s", vkLinkNum, v)
		}
		o4 *= 2
	}

	o2, o3 := ipLastOctet(sysA.IP()), ipLastOctet(sysB.IP())
	if o3 < o2 {
		o2, o3, o4 = o3, o2, o4+1
	}
	ipA, err = netaddr.ParseIPPrefix(fmt.Sprintf("1.%d.%d.%d/31", o2, o3, o4))
	if err != nil {
		log.Errorf("could not create link IP from %s: %s", vkSystemIP, err)
	}
	return ipA.String(), ipFarEnd(ipA).String(), nil
}

func ipLastOctet(in netaddr.IP) int {
	s := in.String()
	i := strings.LastIndexAny(s, ".")
	if i < 0 {
		i = strings.LastIndexAny(s, ":")
	}
	res, err := strconv.Atoi(s[i+1:])
	if err != nil {
		log.Errorf("last octet %s from IP %s not a string", s[i+1:], s)
	}
	return res
}

// Calculates the far end IP (first free IP in the subnet) - string version
func ipFarEndS(in string) (string, error) {
	ipA, err := netaddr.ParseIPPrefix(in)
	if err != nil {
		return "", fmt.Errorf("invalid ip %s", in)
	}
	return ipFarEnd(ipA).String(), nil
}

// Calculates the far end IP (first free IP in the subnet)
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

// GetTemplateNamesInDirs returns a list of template file names found in a list of dir `paths`
// without traversing nested dirs
// template names are following the pattern <some-name>__<role/kind>.tmpl
func GetTemplateNamesInDirs(paths []string) ([]string, error) {
	var tnames []string
	for _, p := range paths {
		all, err := filepath.Glob(filepath.Join(p, "*__*.tmpl"))
		if err != nil {
			return nil, err
		}
		for _, fn := range all {
			tn := strings.Split(filepath.Base(fn), "__")[0]
			// skip adding templates with the same name
			if len(tnames) > 0 && tnames[len(tnames)-1] == tn {
				continue
			}
			tnames = append(tnames, tn)
		}
	}
	return tnames, nil
}
