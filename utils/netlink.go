// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
)

func BridgeByName(name string) (*netlink.Bridge, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("could not lookup %q: %v", name, err)
	}
	br, ok := l.(*netlink.Bridge)
	if !ok {
		return nil, fmt.Errorf("%q already exists but is not a bridge", name)
	}
	return br, nil
}

// linkContainerNS creates a symlink for containers network namespace
// so that it can be managed by iproute2 utility
func LinkContainerNS(nspath, containerName string) error {
	CreateDirectory("/run/netns/", 0755)
	dst := "/run/netns/" + containerName
	err := os.Symlink(nspath, dst)
	if err != nil {
		return err
	}
	return nil
}

// getDefaultDockerMTU gets the MTU of a docker0 bridge interface
// if fails to get the MTU of docker0, returns "1500"
func DefaultNetMTU() (string, error) {
	b, err := BridgeByName("docker0")
	if err != nil {
		return "1500", err
	}
	return fmt.Sprint(b.MTU), nil
}
