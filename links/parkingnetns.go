package links

import (
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netns"
)

// ParkingNetNs represents a simple NetworkNamespace.
// It is created when nodes are meant to be restarted.
// Prior to the restart interfaces will be relocated
// to the ParkingNetNs and after the reboot is completed,
// interfaces get shifted back.
type ParkingNetNs struct {
	netNsFd int
	name    string
}

func NewParkingNetNs(name string) (*ParkingNetNs, error) {
	// store the actual namespace handle
	baseNetNs, err := netns.Get()
	if err != nil {
		return nil, err
	}

	// create a "parking" namespace for the network interfaces
	rebootNs, err := netns.NewNamed(name)
	if err != nil {
		return nil, err
	}

	// revert back to the inital network namespace
	err = netns.Set(baseNetNs)
	if err != nil {
		return nil, err
	}

	return &ParkingNetNs{
		netNsFd: int(rebootNs),
		name:    name,
	}, nil
}

func (p *ParkingNetNs) Delete() error {
	return netns.DeleteNamed(p.name)
}

func (p *ParkingNetNs) GetNetNs() (ns.NetNS, error) {
	return ns.GetNS(fmt.Sprintf("/proc/self/fd/%d", p.netNsFd))
}

func (p *ParkingNetNs) GetFd() int {
	return p.netNsFd
}

func (p *ParkingNetNs) ExecFunction(f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	netns, err := p.GetNetNs()
	if err != nil {
		return err
	}
	// execute the given function
	return netns.Do(f)
}
