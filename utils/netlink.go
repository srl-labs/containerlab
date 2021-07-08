// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"crypto/rand"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
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
	if _, err := os.Lstat(dst); err == nil {
		os.Remove(dst)
	}
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

func CheckBrInUse(brname string) (bool, error) {
	InUse := false
	l, err := netlink.LinkList()
	if err != nil {
		return InUse, err
	}
	mgmtbr, err := netlink.LinkByName(brname)
	if err != nil {
		return InUse, err
	}
	mgmtbridx := mgmtbr.Attrs().Index
	for _, link := range l {
		if link.Attrs().MasterIndex == mgmtbridx {
			InUse = true
			break
		}
	}
	return InUse, nil
}

func DeleteLinkByName(name string) error {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkDel(l)
}

// GenMac generates a random MAC address for a given OUI
func GenMac(oui string) string {
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s:%02x:%02x:%02x", oui, buf[0], buf[1], buf[2])
}

// deleteNetnsSymlink deletes a network namespace and removes the symlink created by linkContainerNS func
func DeleteNetnsSymlink(n string) error {
	log.Debug("Deleting netns symlink: ", n)
	sl := fmt.Sprintf("/run/netns/%s", n)
	err := os.Remove(sl)
	if err != nil {
		log.Debug("Failed to delete netns symlink by path:", sl)
	}
	return nil
}
