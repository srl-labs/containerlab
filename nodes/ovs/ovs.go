// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ovs

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	goOvs "github.com/digitalocean/go-openvswitch/ovs"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabinternalslices "github.com/srl-labs/containerlab/internal/slices"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

const ovsEndpointExtID = "containerlab-endpoint"

// clabAutoPort matches the random netdev name used when the topology interface name is not a
// valid Linux name. Those netdevs are what Open vSwitch records as the port name.
var clabAutoPort = regexp.MustCompile(`^clab-[0-9a-f]{8}$`)

var kindNames = []string{"ovs-bridge"}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	r.Register(kindNames, func() clabnodes.Node {
		return new(ovs)
	}, nil)
}

type ovs struct {
	clabnodes.DefaultNode
}

func (n *ovs) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	n.Cfg.IsRootNamespaceBased = true

	return nil
}

func (n *ovs) CheckDeploymentConditions(_ context.Context) error {
	// check if ovs bridge exists
	c := goOvs.New()

	// We were previously doing c.VSwitch.Get.Bridge() but it doesn't work
	// when the bridge has a protocol version higher than 1.0
	// So listing the bridges is safer
	bridges, err := c.VSwitch.ListBridges()
	if err != nil {
		return fmt.Errorf("error while listing ovs bridges: %v", err)
	}
	if !clabinternalslices.Contains(bridges, n.Cfg.ShortName) {
		return fmt.Errorf("could not find ovs bridge %q", n.Cfg.ShortName)
	}

	return nil
}

func (n *ovs) Deploy(_ context.Context, _ *clabnodes.DeployParams) error {
	n.SetState(clabnodesstate.Deployed)
	return nil
}

func (*ovs) PullImage(_ context.Context) error             { return nil }
func (*ovs) GetImages(_ context.Context) map[string]string { return map[string]string{} }

func (n *ovs) Delete(_ context.Context) error {
	for _, ep := range n.GetEndpoints() {
		if err := n.DeleteBridgePort(ep.GetIfaceName()); err != nil {
			log.Errorf(
				"Could not remove OVS port %q from bridge %q: %v",
				ep.GetIfaceName(),
				n.Config().ShortName,
				err,
			)
		}
	}

	// Ports created before they were tagged, or whose netdev disappeared with the container,
	// are not named after the topology interface. Drop those leftovers too.
	if err := n.removeMissingAutoPorts(); err != nil {
		log.Errorf(
			"Could not remove stale OVS ports from bridge %q: %v",
			n.Config().ShortName,
			err,
		)
	}

	return nil
}

// DeleteBridgePort removes the OVS port for a topology interface. The port name matches the
// topology name only when that name is a valid Linux interface name. Otherwise Open vSwitch keeps
// the random clab-* netdev name, which has to be found via the tag recorded at attach time.
func (n *ovs) DeleteBridgePort(endpoint string) error {
	if err := n.removeEndpointPorts(endpoint); err != nil {
		return err
	}

	return n.removeMissingAutoPorts()
}

func (*ovs) DeleteNetnsSymlink() (err error) { return nil }

// UpdateConfigWithRuntimeInfo is a noop for bridges.
func (*ovs) UpdateConfigWithRuntimeInfo(_ context.Context) error { return nil }

// GetContainers is a noop for bridges.
func (*ovs) GetContainers(_ context.Context) ([]clabruntime.GenericContainer, error) {
	return nil, nil
}

// RunExec is noop for ovs kind.
func (n *ovs) RunExec(_ context.Context, _ *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
	log.Warnf("Exec operation is not implemented for kind %q", n.Config().Kind)

	return nil, clabexec.ErrRunExecNotSupported
}

func (n *ovs) AddLinkToContainer(
	ctx context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
	// retrieve the namespace handle
	curNamespace, err := ns.GetCurrentNS()
	if err != nil {
		return err
	}

	// execute the given function
	err = curNamespace.Do(f)
	if err != nil {
		return err
	}

	c := goOvs.New()

	// need to reread the link, due to the changed the function f might have applied to
	// the link. Usually it renames the link to its final name.
	link, err = netlink.LinkByIndex(link.Attrs().Index)
	if err != nil {
		return err
	}

	logical := n.logicalEndpointName(link)
	// A previous attach of this same topology interface may still be registered under the random
	// netdev name. Remove it before adding the new one so redeploy does not accumulate ports.
	if err := n.DeleteBridgePort(logical); err != nil {
		return err
	}

	if err := c.VSwitch.AddPort(n.Cfg.ShortName, link.Attrs().Name); err != nil {
		return err
	}

	if logical == "" || logical == link.Attrs().Name {
		return nil
	}

	return setOVSEndpointID(link.Attrs().Name, logical)
}

func (m *ovs) GetLinkEndpointType() clablinks.LinkEndpointType {
	return clablinks.LinkEndpointTypeBridge
}

// logicalEndpointName is the topology interface name for link. When that name is not a valid
// Linux interface name, the netdev keeps a random clab-* name and the topology name is an altname.
func (n *ovs) logicalEndpointName(link netlink.Link) string {
	if link == nil {
		return ""
	}

	name := link.Attrs().Name
	alts := make(map[string]struct{}, len(link.Attrs().AltNames))
	for _, alt := range link.Attrs().AltNames {
		alts[alt] = struct{}{}
	}

	for _, ep := range n.GetEndpoints() {
		iface := ep.GetIfaceName()
		if iface == name {
			return iface
		}
		if _, ok := alts[clabutils.SanitizeInterfaceName(iface)]; ok {
			return iface
		}
	}

	return name
}

func (n *ovs) removeEndpointPorts(endpoint string) error {
	if endpoint == "" || n.Cfg == nil || n.Cfg.ShortName == "" {
		return nil
	}

	c := goOvs.New()
	// Valid Linux names are used as the OVS port name. --if-exists covers a missing port.
	if err := c.VSwitch.DeletePort(n.Cfg.ShortName, endpoint); err != nil {
		return err
	}

	names, err := ovsInterfaceNames(
		"external_ids:" + ovsEndpointExtID + "=" + ovsMapValue(endpoint),
	)
	if err != nil {
		return err
	}

	onBridge, err := bridgePortSet(c, n.Cfg.ShortName)
	if err != nil {
		return err
	}

	for _, name := range names {
		if _, ok := onBridge[name]; !ok {
			continue
		}
		if err := c.VSwitch.DeletePort(n.Cfg.ShortName, name); err != nil {
			return err
		}
	}

	return nil
}

// removeMissingAutoPorts deletes clab-* ports whose netdev is already gone. Deleting the container
// side of the veth removes the device and leaves the OVS port in place under that random name.
func (n *ovs) removeMissingAutoPorts() error {
	if n.Cfg == nil || n.Cfg.ShortName == "" {
		return nil
	}

	c := goOvs.New()
	ports, err := c.VSwitch.ListPorts(n.Cfg.ShortName)
	if err != nil {
		return err
	}

	for _, port := range ports {
		if !clabAutoPort.MatchString(port) {
			continue
		}
		missing, err := ovsInterfaceDeviceMissing(port)
		if err != nil {
			return err
		}
		if !missing {
			continue
		}
		log.Debugf(
			"Removing stale OVS port %q from bridge %q",
			port,
			n.Cfg.ShortName,
		)
		if err := c.VSwitch.DeletePort(n.Cfg.ShortName, port); err != nil {
			return err
		}
	}

	return nil
}

func bridgePortSet(c *goOvs.Client, bridge string) (map[string]struct{}, error) {
	ports, err := c.VSwitch.ListPorts(bridge)
	if err != nil {
		return nil, err
	}

	set := make(map[string]struct{}, len(ports))
	for _, port := range ports {
		set[port] = struct{}{}
	}

	return set, nil
}

func setOVSEndpointID(iface, endpoint string) error {
	out, err := exec.Command(
		"ovs-vsctl",
		"set",
		"Interface",
		iface,
		"external_ids:"+ovsEndpointExtID+"="+ovsMapValue(endpoint),
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tag OVS interface %q as %q: %w (%s)", iface, endpoint, err, out)
	}

	return nil
}

func ovsInterfaceNames(condition string) ([]string, error) {
	out, err := exec.Command(
		"ovs-vsctl",
		"--format=csv",
		"--no-headings",
		"--data=bare",
		"--columns=name",
		"find",
		"Interface",
		condition,
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("find OVS interfaces: %w (%s)", err, out)
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}

	return strings.Split(text, "\n"), nil
}

func ovsInterfaceDeviceMissing(iface string) (bool, error) {
	out, err := exec.Command(
		"ovs-vsctl",
		"--data=bare",
		"get",
		"Interface",
		iface,
		"error",
	).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("get OVS interface %q error: %w (%s)", iface, err, out)
	}

	return ovsDeviceMissing(string(out)), nil
}

// ovsDeviceMissing reports whether an OVS interface error column says the netdev is gone.
// A live interface returns "[]".
func ovsDeviceMissing(errorColumn string) bool {
	errorColumn = strings.TrimSpace(errorColumn)

	return strings.Contains(errorColumn, "could not open network device")
}

func ovsMapValue(value string) string {
	if value == "" {
		return `""`
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("-_.:@", r):
		default:
			return fmt.Sprintf("%q", value)
		}
	}

	return value
}
