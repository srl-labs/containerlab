// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ip

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/coreos/go-iptables/iptables"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/utils"
)

// setupIPMasqIPTables is the iptables-based implementation of SetupIPMasqForNetworks
func setupIPMasqIPTables(ipns []*net.IPNet, network, _, containerID string) error {
	// Note: for historical reasons, the iptables implementation ignores ifname.
	chain := utils.FormatChainName(network, containerID)
	comment := utils.FormatComment(network, containerID)
	for _, ip := range ipns {
		if err := SetupIPMasq(ip, chain, comment); err != nil {
			return err
		}
	}
	return nil
}

// SetupIPMasq installs iptables rules to masquerade traffic
// coming from ip of ipn and going outside of ipn.
// Deprecated: This function only supports iptables. Use SetupIPMasqForNetworks, which
// supports both iptables and nftables.
func SetupIPMasq(ipn *net.IPNet, chain string, comment string) error {
	isV6 := ipn.IP.To4() == nil

	var ipt *iptables.IPTables
	var err error
	var multicastNet string

	if isV6 {
		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv6)
		multicastNet = "ff00::/8"
	} else {
		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv4)
		multicastNet = "224.0.0.0/4"
	}
	if err != nil {
		return fmt.Errorf("failed to locate iptables: %v", err)
	}

	// Create chain if doesn't exist
	exists := false
	chains, err := ipt.ListChains("nat")
	if err != nil {
		return fmt.Errorf("failed to list chains: %v", err)
	}
	for _, ch := range chains {
		if ch == chain {
			exists = true
			break
		}
	}
	if !exists {
		if err = ipt.NewChain("nat", chain); err != nil {
			return err
		}
	}

	// Packets to this network should not be touched
	if err := ipt.AppendUnique("nat", chain, "-d", ipn.String(), "-j", "ACCEPT", "-m", "comment", "--comment", comment); err != nil {
		return err
	}

	// Don't masquerade multicast - pods should be able to talk to other pods
	// on the local network via multicast.
	if err := ipt.AppendUnique("nat", chain, "!", "-d", multicastNet, "-j", "MASQUERADE", "-m", "comment", "--comment", comment); err != nil {
		return err
	}

	// Packets from the specific IP of this network will hit the chain
	return ipt.AppendUnique("nat", "POSTROUTING", "-s", ipn.IP.String(), "-j", chain, "-m", "comment", "--comment", comment)
}

// teardownIPMasqIPTables is the iptables-based implementation of TeardownIPMasqForNetworks
func teardownIPMasqIPTables(ipns []*net.IPNet, network, _, containerID string) error {
	// Note: for historical reasons, the iptables implementation ignores ifname.
	chain := utils.FormatChainName(network, containerID)
	comment := utils.FormatComment(network, containerID)

	var errs []string
	for _, ipn := range ipns {
		err := TeardownIPMasq(ipn, chain, comment)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}

// TeardownIPMasq undoes the effects of SetupIPMasq.
// Deprecated: This function only supports iptables. Use TeardownIPMasqForNetworks, which
// supports both iptables and nftables.
func TeardownIPMasq(ipn *net.IPNet, chain string, comment string) error {
	isV6 := ipn.IP.To4() == nil

	var ipt *iptables.IPTables
	var err error

	if isV6 {
		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv6)
	} else {
		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv4)
	}
	if err != nil {
		return fmt.Errorf("failed to locate iptables: %v", err)
	}

	err = ipt.Delete("nat", "POSTROUTING", "-s", ipn.IP.String(), "-j", chain, "-m", "comment", "--comment", comment)
	if err != nil && !isNotExist(err) {
		return err
	}

	// for downward compatibility
	err = ipt.Delete("nat", "POSTROUTING", "-s", ipn.String(), "-j", chain, "-m", "comment", "--comment", comment)
	if err != nil && !isNotExist(err) {
		return err
	}

	err = ipt.ClearChain("nat", chain)
	if err != nil && !isNotExist(err) {
		return err
	}

	err = ipt.DeleteChain("nat", chain)
	if err != nil && !isNotExist(err) {
		return err
	}

	return nil
}

// gcIPMasqIPTables is the iptables-based implementation of GCIPMasqForNetwork
func gcIPMasqIPTables(_ string, _ []types.GCAttachment) error {
	// FIXME: The iptables implementation does not support GC.
	//
	// (In theory, it _could_ backward-compatibly support it, by adding a no-op rule
	// with a comment indicating the network to each chain it creates, so that it
	// could later figure out which chains corresponded to which networks; older
	// implementations would ignore the extra rule but would still correctly delete
	// the chain on teardown (because they ClearChain() before doing DeleteChain()).

	return nil
}

// isNotExist returnst true if the error is from iptables indicating
// that the target does not exist.
func isNotExist(err error) bool {
	e, ok := err.(*iptables.Error)
	if !ok {
		return false
	}
	return e.IsNotExist()
}
