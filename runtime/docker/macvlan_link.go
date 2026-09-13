package docker

import (
	"github.com/srl-labs/containerlab/mgmt"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

func (d *DockerRuntime) macvlanHost() clabutils.MacvlanHost {
	host := clabutils.MacvlanHost{Links: d.macvlanNetlink}
	if host.Links == nil {
		host.Links = &netlink.Handle{}
	}
	if d.mgmt.IPAM.Provider != clabtypes.IPAMProviderRuntime && d.mgmt.IPAM.DADEnabled() &&
		d.mgmt.Driver == "macvlan" && d.mgmt.EffectiveMacvlanMode() == "bridge" {
		host.Probe = mgmt.ProbeAddress
	}
	return host
}
