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

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/utils"
)

// SetupIPMasqForNetworks installs rules to masquerade traffic coming from ips of ipns and
// going outside of ipns, using a chain name based on network, ifname, and containerID. The
// backend can be either "iptables" or "nftables"; if it is nil, then a suitable default
// implementation will be used.
func SetupIPMasqForNetworks(backend *string, ipns []*net.IPNet, network, ifname, containerID string) error {
	if backend == nil {
		// Prefer iptables, unless only nftables is available
		defaultBackend := "iptables"
		if !utils.SupportsIPTables() && utils.SupportsNFTables() {
			defaultBackend = "nftables"
		}
		backend = &defaultBackend
	}

	switch *backend {
	case "iptables":
		return setupIPMasqIPTables(ipns, network, ifname, containerID)
	case "nftables":
		return setupIPMasqNFTables(ipns, network, ifname, containerID)
	default:
		return fmt.Errorf("unknown ipmasq backend %q", *backend)
	}
}

// TeardownIPMasqForNetworks undoes the effects of SetupIPMasqForNetworks
func TeardownIPMasqForNetworks(ipns []*net.IPNet, network, ifname, containerID string) error {
	var errs []string

	// Do both the iptables and the nftables cleanup, since the pod may have been
	// created with a different version of this plugin or a different configuration.

	err := teardownIPMasqIPTables(ipns, network, ifname, containerID)
	if err != nil && utils.SupportsIPTables() {
		errs = append(errs, err.Error())
	}

	err = teardownIPMasqNFTables(ipns, network, ifname, containerID)
	if err != nil && utils.SupportsNFTables() {
		errs = append(errs, err.Error())
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}

// GCIPMasqForNetwork garbage collects stale IPMasq entries for network
func GCIPMasqForNetwork(network string, attachments []types.GCAttachment) error {
	var errs []string

	err := gcIPMasqIPTables(network, attachments)
	if err != nil && utils.SupportsIPTables() {
		errs = append(errs, err.Error())
	}

	err = gcIPMasqNFTables(network, attachments)
	if err != nil && utils.SupportsNFTables() {
		errs = append(errs, err.Error())
	}

	if errs == nil {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}
