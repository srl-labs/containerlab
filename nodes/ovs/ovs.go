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

const (
	ovsEndpointExtID = "containerlab-endpoint"
	// ovsTimeoutSeconds bounds ovs-vsctl calls, which wait forever when ovs-vswitchd is down.
	ovsTimeoutSeconds = 30
)

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
	c := goOvs.New(goOvs.Timeout(ovsTimeoutSeconds))

	for _, ep := range n.GetEndpoints() {
		// Under the hood, this is called with "--if-exists", so it will handle the case where it
		// doesn't exist for some reason.
		err := c.VSwitch.DeletePort(n.Cfg.ShortName, ep.GetIfaceName())
		if err == nil {
			err = n.removeEndpointPorts(ep.GetIfaceName())
		}
		if err != nil {
			log.Errorf(
				"Could not remove OVS port %q from bridge %q: %v",
				ep.GetIfaceName(),
				n.Config().ShortName,
				err,
			)
		}
	}

	// One sweep for ports created before they were tagged. Per-endpoint deletion already
	// removes ports this version tagged at attach time.
	if err := n.removeMissingAutoPorts(); err != nil {
		log.Errorf(
			"Could not remove stale OVS ports from bridge %q: %v",
			n.Config().ShortName,
			err,
		)
	}

	return nil
}

// DeleteBridgePort removes the OVS port containerlab tagged for one topology interface. Untagged
// ports may belong to the operator, so only Delete removes a port by its name.
func (n *ovs) DeleteBridgePort(endpoint string) error {
	return n.removeEndpointPorts(endpoint)
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

	// need to reread the link, due to the changed the function f might have applied to
	// the link. Usually it renames the link to its final name.
	link, err = netlink.LinkByIndex(link.Attrs().Index)
	if err != nil {
		return err
	}

	name := link.Attrs().Name
	logical := n.logicalEndpointName(link)
	args := []string{"--may-exist", "add-port", n.Cfg.ShortName, name}
	// A port named after the interface is reused by --may-exist and keeps its config. A random
	// netdev name needs a tag, so that a later attach or link removal can find the port. The full
	// stale-port sweep runs once from Delete, not on every attach.
	if logical != name {
		if err := n.removeEndpointPorts(logical); err != nil {
			return err
		}
		args = append(args,
			"--", "set", "Interface", name,
			"external_ids:"+ovsEndpointExtID+"="+ovsMapValue(logical),
		)
	}

	out, err := ovsVsctl(args...)
	if err != nil {
		return fmt.Errorf("add OVS port %q as %q: %w (%s)", name, logical, err, out)
	}

	return nil
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

// removeEndpointPorts deletes the ports on this bridge tagged with the topology interface name.
func (n *ovs) removeEndpointPorts(endpoint string) error {
	if endpoint == "" || n.Cfg == nil || n.Cfg.ShortName == "" {
		return nil
	}

	c := goOvs.New(goOvs.Timeout(ovsTimeoutSeconds))

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

	c := goOvs.New(goOvs.Timeout(ovsTimeoutSeconds))
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

func ovsVsctl(args ...string) ([]byte, error) {
	args = append([]string{fmt.Sprintf("--timeout=%d", ovsTimeoutSeconds)}, args...)

	return exec.Command("ovs-vsctl", args...).CombinedOutput()
}

func ovsInterfaceNames(condition string) ([]string, error) {
	out, err := ovsVsctl(
		"--format=csv",
		"--no-headings",
		"--data=bare",
		"--columns=name",
		"find",
		"Interface",
		condition,
	)
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
	out, err := ovsVsctl("--data=bare", "get", "Interface", iface, "error")
	if err != nil {
		// The port disappeared between ListPorts and this get. Nothing left to delete.
		if ovsRowMissing(string(out)) {
			return false, nil
		}
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

func ovsRowMissing(output string) bool {
	return strings.Contains(strings.ToLower(output), "no row")
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
